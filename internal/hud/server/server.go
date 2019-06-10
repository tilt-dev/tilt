package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
)

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
	numWebsocketConns int32
}

func ProvideHeadsUpServer(store *store.Store, assetServer assets.Server, analytics *tiltanalytics.TiltAnalytics, sailCli client.SailClient) *HeadsUpServer {
	r := mux.NewRouter().UseEncodedPath()
	s := &HeadsUpServer{
		store:   store,
		router:  r,
		a:       analytics,
		sailCli: sailCli,
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/api/analytics", s.HandleAnalytics)
	r.HandleFunc("/api/analytics_opt", s.HandleAnalyticsOpt)
	r.HandleFunc("/api/sail", s.HandleSail)
	r.HandleFunc("/api/trigger", s.HandleTrigger)
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
