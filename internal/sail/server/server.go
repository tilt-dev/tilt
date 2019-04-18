package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	hudServer "github.com/windmilleng/tilt/internal/hud/server"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SailServer struct {
	router      *mux.Router
	rooms       map[RoomID]*Room
	mu          *sync.Mutex
	assetServer hudServer.AssetServer
}

func ProvideSailServer(assetServer hudServer.AssetServer) SailServer {
	r := mux.NewRouter().UseEncodedPath()
	s := SailServer{
		router:      r,
		rooms:       make(map[RoomID]*Room, 0),
		mu:          &sync.Mutex{},
		assetServer: assetServer,
	}

	r.HandleFunc("/new_room", s.newRoom)
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
	w.Write([]byte(room.id))
	log.Printf("newRoom: %s", room.id)
}

func (s SailServer) hasRoom(roomID RoomID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.rooms[roomID]
	return ok
}

func (s SailServer) getRoomWithAuth(roomID RoomID, secret string) (*Room, error) {
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
	spew.Dump(req)
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("connectRoom: %v", err)
		return
	}

	room, err := s.getRoomWithAuth(RoomID("boop"), "shh")
	if err != nil {
		log.Printf("connectRoom: %v", err)
		return
	}
	room.source = conn

	err = room.ConsumeSource(req.Context())
	if err != nil {
		log.Printf("websocket closed: %v", err)
	}

	s.closeRoom(room)
}

func (s SailServer) addFanToRoom(ctx context.Context, roomID RoomID, conn *websocket.Conn) error {
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
	roomID := RoomID(vars["roomID"])
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
	roomID := RoomID(vars["roomID"])
	if !s.hasRoom(roomID) {
		http.Error(w, fmt.Sprintf("Room not found: %q", roomID), http.StatusNotFound)
		return
	}

	req.URL.Path = "/index.html"
	s.assetServer.ServeHTTP(w, req)
}
