package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/gorilla/websocket"
)

var upgrader = newUpgrader()

func newUpgrader() websocket.Upgrader {
	result := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	result.CheckOrigin = checkWebsocketOrigin
	return result
}

type WebsocketSubscriber struct {
	conn *websocket.Conn
}

func NewWebsocketSubscriber(conn *websocket.Conn) WebsocketSubscriber {
	return WebsocketSubscriber{
		conn: conn,
	}
}

func (ws WebsocketSubscriber) Stream(ctx context.Context, store *store.Store) {
	done := make(chan bool)

	go func() {
		// No-op consumption of all control messages, as recommended here:
		// https://godoc.org/github.com/gorilla/websocket#hdr-Control_Messages
		conn := ws.conn
		for {
			if _, _, err := conn.NextReader(); err != nil {
				_ = conn.Close()
				close(done)
				break
			}
		}
	}()

	store.AddSubscriber(ws)

	// Fire a fake OnChange event to initialize the stream.
	ws.OnChange(ctx, store)

	<-done
	_ = store.RemoveSubscriber(ws)
}

func (ws WebsocketSubscriber) OnChange(ctx context.Context, s store.RStore) {
	state := s.RLockState()
	view := store.StateToView(state)
	s.RUnlockState()

	err := ws.conn.WriteJSON(view)
	if err != nil {
		_ = ws.conn.Close()
	}
}

func (s HeadsUpServer) ViewWebsocket(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error upgrading websocket: %v", err), http.StatusInternalServerError)
		return
	}

	ws := NewWebsocketSubscriber(conn)

	// TODO(nick): Handle clean shutdown when the server shuts down
	ws.Stream(context.TODO(), s.store)
}
