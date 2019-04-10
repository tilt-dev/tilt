package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SailServer struct {
	router *mux.Router
	rooms  map[RoomID]*Room
	mu     *sync.Mutex
}

func ProvideSailServer() SailServer {
	r := mux.NewRouter().UseEncodedPath()
	s := SailServer{
		router: r,
		rooms:  make(map[RoomID]*Room, 0),
		mu:     &sync.Mutex{},
	}

	r.HandleFunc("/share", s.startRoom)

	return s
}

func (s SailServer) Router() http.Handler {
	return s.router
}

func (s SailServer) newRoom(conn *websocket.Conn) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	room := NewRoom(conn)
	s.rooms[room.id] = room
	return room
}

func (s SailServer) closeRoom(room *Room) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.rooms, room.id)
}

func (s SailServer) startRoom(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("startRoom: %v", err)
		return
	}

	room := s.newRoom(conn)
	err = room.ConsumeSource(req.Context())
	if err != nil {
		log.Printf("websocket closed: %v", err)
	}

	s.closeRoom(room)
}
