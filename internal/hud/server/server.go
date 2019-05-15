package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type analyticsPayload struct {
	Verb string            `json:"verb"`
	Name string            `json:"name"`
	Tags map[string]string `json:"tags"`
}

type analyticsOptPayload struct {
	Opt string `json:"opt"`
}

type HeadsUpServer struct {
	store             *store.Store
	router            *mux.Router
	a                 analytics.Analytics
	sailCli           client.SailClient
	opter             AnalyticsOpter
	numWebsocketConns int32
}

func ProvideHeadsUpServer(store *store.Store, assetServer assets.Server, analytics analytics.Analytics, sailCli client.SailClient, opter AnalyticsOpter) *HeadsUpServer {
	r := mux.NewRouter().UseEncodedPath()
	s := &HeadsUpServer{
		store:   store,
		router:  r,
		a:       analytics,
		sailCli: sailCli,
		opter:   opter,
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/api/analytics", s.HandleAnalytics)
	r.HandleFunc("/api/analytics_opt", s.HandleAnalyticsOpt)
	r.HandleFunc("/api/sail", s.HandleSail)
	r.HandleFunc("/api/control/reset_restarts", s.HandleResetRestarts)
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

	err = s.opter.SetOptStr(payload.Opt)
	if err != nil {
		http.Error(w, fmt.Sprintf("error setting opt '%s': %v", payload.Opt, err), http.StatusInternalServerError)
		return
	}
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
		s.store.Dispatch(store.LogAction{
			LogEvent: store.LogEvent{
				Timestamp: time.Now(),
				Msg:       append([]byte{}, b...),
			},
		})
		return nil
	})

	err := s.sailCli.Connect(logger.WithLogger(req.Context(), l), s.store)
	if err != nil {
		log.Printf("sailClient.NewRoom: %v", err)
		http.Error(w, fmt.Sprintf("error creating new Sail room: %v", err), http.StatusInternalServerError)
		return
	}

}

func (s *HeadsUpServer) HandleResetRestarts(w http.ResponseWriter, req *http.Request) {
	names, ok := req.URL.Query()["name"]
	if !ok || len(names[0]) < 1 {
		http.Error(w, "Must contain name parameter", http.StatusBadRequest)
		return
	}

	// Query()["name"] will return an array of items,
	// we only want the single item.
	name := names[0]

	s.store.Dispatch(store.NewResetRestartsAction(model.ManifestName(name)))
	w.WriteHeader(http.StatusOK)
}
