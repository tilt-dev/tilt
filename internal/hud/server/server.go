package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type analyticsPayload struct {
	Verb string            `json:"verb"`
	Name string            `json:"name"`
	Tags map[string]string `json:"tags"`
}

type HeadsUpServer struct {
	store  *store.Store
	router *mux.Router
	a      analytics.Analytics
}

func ProvideHeadsUpServer(store *store.Store, assetServer assets.Server, analytics analytics.Analytics) HeadsUpServer {
	r := mux.NewRouter().UseEncodedPath()
	s := HeadsUpServer{
		store:  store,
		router: r,
		a:      analytics,
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/api/analytics", s.HandleAnalytics)
	r.HandleFunc("/ws/view", s.ViewWebsocket)
	r.PathPrefix("/").Handler(assetServer)

	return s
}

func (s HeadsUpServer) Router() http.Handler {
	return s.router
}

func (s HeadsUpServer) ViewJSON(w http.ResponseWriter, req *http.Request) {
	state := s.store.RLockState()
	view := StateToWebView(state)
	s.store.RUnlockState()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(view)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering view payload: %v", err), http.StatusInternalServerError)
	}
}

func (s HeadsUpServer) HandleAnalytics(w http.ResponseWriter, req *http.Request) {
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
