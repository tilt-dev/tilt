package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/model"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SailServer struct {
	router      *mux.Router
	rooms       map[model.RoomID]*Room
	mu          *sync.Mutex
	assetServer assets.Server
}

func ProvideSailServer(assetServer assets.Server) SailServer {
	r := mux.NewRouter().UseEncodedPath()
	s := SailServer{
		router:      r,
		rooms:       make(map[model.RoomID]*Room, 0),
		mu:          &sync.Mutex{},
		assetServer: assetServer,
	}

	r.HandleFunc("/room", s.newRoom) // currently expects only POST requests, in future this endpt. might do things other than create a new room
	r.HandleFunc("/share", s.connectRoom)
	r.HandleFunc("/join/{roomID}", s.joinRoom)
	r.HandleFunc("/view/{roomID}", s.viewRoom)
	r.PathPrefix("/").Handler(assetServer)

	return s
}

func (s SailServer) Router() http.Handler {
	return s.router
}

func (s SailServer) newRoom(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room := NewRoom()
	s.rooms[room.id] = room

	resp, err := room.newRoomResponse()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf("error creating newRoom response: %v", err)))
	}

	_, _ = w.Write(resp)
	log.Printf("newRoom: %s", room.id)
}

func (s SailServer) hasRoom(roomID model.RoomID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.rooms[roomID]
	return ok
}

func (s SailServer) getRoomWithAuth(roomID model.RoomID, secret string) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil, fmt.Errorf("getRoomWithAuth: no room found with ID: %s", roomID)
	}

	if room.secret != secret {
		return nil, fmt.Errorf("getRoomWithAuth: incorrect secret for room %s", roomID)
	}

	return room, nil
}

func (s SailServer) closeRoom(room *Room) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("closeRoom: %s", room.id)
	delete(s.rooms, room.id)
	room.Close()
}

func (s SailServer) connectRoom(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("connectRoom: %v", err)
		return
	}

	roomID := req.URL.Query().Get(model.SailRoomIDKey)
	secret := req.Header.Get(model.SailSecretKey)

	room, err := s.getRoomWithAuth(model.RoomID(roomID), secret)
	if err != nil {
		log.Printf("connectRoom: %v", err)
		w.WriteHeader(http.StatusForbidden) // TODO(maia): send 404 instead if room not found
		_, _ = w.Write([]byte(fmt.Sprintf("error connecting to room %s: %v", roomID, err)))
		return
	}
	room.source = conn

	err = room.ConsumeSource(req.Context())
	if err != nil {
		log.Printf("websocket closed: %v", err)
	}

	s.closeRoom(room)
}

func (s SailServer) addFanToRoom(ctx context.Context, roomID model.RoomID, conn *websocket.Conn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return fmt.Errorf("Room not found: %s", roomID)
	}

	log.Printf("addFanToRoom: %s", room.id)
	room.AddFan(ctx, conn)
	return nil
}

func (s SailServer) joinRoom(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	roomID := model.RoomID(vars["roomID"])
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error upgrading websocket: %v", err), http.StatusInternalServerError)
		return
	}

	err = s.addFanToRoom(req.Context(), roomID, conn)
	if err != nil {
		log.Printf("Room add error: %v", err)
		return
	}
}

func (s SailServer) viewRoom(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	roomID := model.RoomID(vars["roomID"])
	if !s.hasRoom(roomID) {
		http.Error(w, fmt.Sprintf("Room not found: %q", roomID), http.StatusNotFound)
		return
	}

	req.URL.Path = "/index.html"
	s.assetServer.ServeHTTP(w, req)
}
