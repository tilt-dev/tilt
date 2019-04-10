package server

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SailServer struct {
	router *mux.Router
}

func ProvideSailServer() SailServer {
	r := mux.NewRouter().UseEncodedPath()
	s := SailServer{
		router: r,
	}

	r.HandleFunc("/share", s.startRoom)

	return s
}

func (s SailServer) Router() http.Handler {
	return s.router
}

func (s SailServer) startRoom(w http.ResponseWriter, req *http.Request) {
	_, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("startRoom: %v", err)
		return
	}

	log.Println("Hurray! Connected")
}
