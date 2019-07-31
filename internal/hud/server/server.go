package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
	tft "github.com/windmilleng/tilt/internal/tft/client"
)

const tiltAlertsDomain = "alerts.tilt.dev"
const httpTimeOut = 5 * time.Second

type analyticsPayload struct {
	Verb string            `json:"verb"`
	Name string            `json:"name"`
	Tags map[string]string `json:"tags"`
}

type analyticsOptPayload struct {
	Opt string `json:"opt"`
}

type triggerPayload struct {
	ManifestNames []string `json:"manifest_names"`
}

type HeadsUpServer struct {
	store             *store.Store
	router            *mux.Router
	a                 *tiltanalytics.TiltAnalytics
	sailCli           client.SailClient
	tftCli            tft.Client
	numWebsocketConns int32
}

func ProvideHeadsUpServer(store *store.Store, assetServer assets.Server, analytics *tiltanalytics.TiltAnalytics, sailCli client.SailClient, tftClient tft.Client) *HeadsUpServer {
	r := mux.NewRouter().UseEncodedPath()
	s := &HeadsUpServer{
		store:   store,
		router:  r,
		a:       analytics,
		sailCli: sailCli,
		tftCli:  tftClient,
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/api/analytics", s.HandleAnalytics)
	r.HandleFunc("/api/analytics_opt", s.HandleAnalyticsOpt)
	r.HandleFunc("/api/sail", s.HandleSail)
	r.HandleFunc("/api/trigger", s.HandleTrigger)
	r.HandleFunc("/api/alerts/new", s.HandleNewAlert)
	r.HandleFunc("/ws/view", s.ViewWebsocket)
	r.PathPrefix("/").Handler(assetServer)

	return s
}

func (s *HeadsUpServer) Router() http.Handler {
	return s.router
}

func (s *HeadsUpServer) ViewJSON(w http.ResponseWriter, req *http.Request) {
	state := s.store.RLockState()
	view := webview.StateToWebView(state)
	s.store.RUnlockState()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(view)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering view payload: %v", err), http.StatusInternalServerError)
	}
}

func (s *HeadsUpServer) HandleAnalyticsOpt(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payload analyticsOptPayload

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	opt, err := analytics.ParseOpt(payload.Opt)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing opt '%s': %v", payload.Opt, err), http.StatusBadRequest)
	}

	// only logging on opt-in, because, well, opting out means the user just told us not to report data on them!
	if opt == analytics.OptIn {
		s.a.IncrIfUnopted("analytics.opt.in")
	}

	s.store.Dispatch(store.AnalyticsOptAction{Opt: opt})
}

func (s *HeadsUpServer) HandleAnalytics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payloads []analyticsPayload

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payloads)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	for _, p := range payloads {
		if p.Verb != "incr" {
			http.Error(w, "error parsing payloads: only incr verbs are supported", http.StatusBadRequest)
			return
		}

		s.a.Incr(p.Name, p.Tags)
	}
}

func (s *HeadsUpServer) HandleSail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	// Request context doesn't have logger, just slap one on for now.
	l := logger.NewFuncLogger(false, logger.DebugLvl, func(level logger.Level, b []byte) error {
		s.store.Dispatch(store.NewGlobalLogEvent(b))
		return nil
	})

	err := s.sailCli.Connect(logger.WithLogger(req.Context(), l), s.store)
	if err != nil {
		log.Printf("sailClient.NewRoom: %v", err)
		http.Error(w, fmt.Sprintf("error creating new Sail room: %v", err), http.StatusInternalServerError)
		return
	}

}

func (s *HeadsUpServer) HandleTrigger(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payload triggerPayload

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	if len(payload.ManifestNames) != 1 {
		http.Error(w, fmt.Sprintf("/api/trigger currently supports exactly one manifest name, got %d", len(payload.ManifestNames)), http.StatusBadRequest)
		return
	}

	err = MaybeSendToTriggerQueue(s.store, payload.ManifestNames[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func MaybeSendToTriggerQueue(st store.RStore, name string) error {
	mName := model.ManifestName(name)

	state := st.RLockState()
	m, ok := state.Manifest(mName)
	st.RUnlockState()

	if !ok {
		return fmt.Errorf("no manifest found with name '%s'", mName)
	}

	if m.TriggerMode != model.TriggerModeManual {
		return fmt.Errorf("can only trigger updates for manifests of TriggerModeManual")
	}

	st.Dispatch(AppendToTriggerQueueAction{Name: mName})
	return nil
}

type NewAlertResponse struct {
	Url string `json:"url"`
}

func (s *HeadsUpServer) HandleNewAlert(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(req.Body)
	var alert tsAlert
	err := decoder.Decode(&alert)
	if err != nil {
		http.Error(w, fmt.Sprintf("error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeOut)
	defer cancel()
	id, err := s.tftCli.SendAlert(ctx, tsAlertToBackendAlert(alert))
	if err != nil {
		http.Error(w, fmt.Sprintf("error talking to backend: %v", err), http.StatusBadRequest)
		return
	}

	responsePayload := &NewAlertResponse{
		Url: templateAlertURL(id),
	}
	js, err := json.Marshal(responsePayload)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to marshal JSON (%+v) response: %v", responsePayload, err), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(js)
}

func templateAlertURL(id tft.AlertID) string {
	return fmt.Sprintf("https://%s/alert/%s", tiltAlertsDomain, id)
}

type tsAlert struct {
	AlertType    string `json:"alertType"`
	Header       string `json:"header"`
	Msg          string `json:"msg"`
	Timestamp    string `json:"timestamp"`
	ResourceName string `json:"resourceName"`
}

func tsAlertToBackendAlert(alert tsAlert) tft.Alert {
	return tft.Alert{
		AlertType:    alert.AlertType,
		Header:       alert.Header,
		Msg:          alert.Msg,
		RFC3339Time:  alert.Timestamp,
		ResourceName: alert.ResourceName,
	}
}
