package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/windmilleng/tilt/internal/store"
)

type HeadsUpServer struct {
	store  *store.Store
	router *mux.Router
}

func ProvideHeadsUpServer(store *store.Store, assetServer AssetServer) HeadsUpServer {
	r := mux.NewRouter().UseEncodedPath()
	s := HeadsUpServer{
		store:  store,
		router: r,
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/ws/view", s.ViewWebsocket)
	r.PathPrefix("/").Handler(assetServer)

	return s
}

func (s HeadsUpServer) Router() http.Handler {
	return s.router
}

func (s HeadsUpServer) ViewJSON(w http.ResponseWriter, req *http.Request) {
	state := s.store.RLockState()
	view := store.StateToView(state)
	s.store.RUnlockState()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(view)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering view payload: %v", err), http.StatusInternalServerError)
	}
}
