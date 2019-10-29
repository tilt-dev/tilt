package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WebsocketSubscriber struct {
	conn       WebsocketConn
	streamDone chan bool
}

type WebsocketConn interface {
	NextReader() (int, io.Reader, error)
	Close() error
	WriteJSON(v interface{}) error
	NextWriter(messageType int) (io.WriteCloser, error)
}

var _ WebsocketConn = &websocket.Conn{}

func NewWebsocketSubscriber(conn WebsocketConn) WebsocketSubscriber {
	return WebsocketSubscriber{
		conn:       conn,
		streamDone: make(chan bool, 0),
	}
}

func (ws WebsocketSubscriber) TearDown(ctx context.Context) {
	_ = ws.conn.Close()
}

// Should be called exactly once. Consumes messages until the socket closes.
func (ws WebsocketSubscriber) Stream(ctx context.Context, store *store.Store) {
	go func() {
		// No-op consumption of all control messages, as recommended here:
		// https://godoc.org/github.com/gorilla/websocket#hdr-Control_Messages
		conn := ws.conn
		for {
			if _, _, err := conn.NextReader(); err != nil {
				close(ws.streamDone)
				break
			}
		}
	}()

	<-ws.streamDone

	// When we remove ourselves as a subscriber, the Store waits for any outstanding OnChange
	// events to complete, then calls TearDown.
	_ = store.RemoveSubscriber(context.Background(), ws)
}

func (ws WebsocketSubscriber) OnChange(ctx context.Context, s store.RStore) {
	state := s.RLockState()
	view, err := webview.StateToProtoView(state)
	if err != nil {
		logger.Get(ctx).Infof("error converting view to proto for websocket: %v", err)
		return
	}

	if view.NeedsAnalyticsNudge && !state.AnalyticsNudgeSurfaced {
		// If we're showing the nudge and no one's told the engine
		// state about it yet... tell the engine state.
		s.Dispatch(store.AnalyticsNudgeSurfacedAction{})
	}
	s.RUnlockState()

	jsEncoder := &runtime.JSONPb{OrigName: false, EmitDefaults: true}
	w, err := ws.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		logger.Get(ctx).Verbosef("getting writer: %v", err)
	}
	defer func() {
		err := w.Close()
		if err != nil {
			logger.Get(ctx).Verbosef("error closing websocket: %v", err)
		}
	}()

	err = jsEncoder.NewEncoder(w).Encode(view)
	if err != nil {
		logger.Get(ctx).Verbosef("sending webview data: %v", err)
	}
}

func (s *HeadsUpServer) ViewWebsocket(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error upgrading websocket: %v", err), http.StatusInternalServerError)
		return
	}

	atomic.AddInt32(&s.numWebsocketConns, 1)
	ws := NewWebsocketSubscriber(conn)

	// TODO(nick): Handle clean shutdown when the server shuts down
	ctx := context.TODO()

	// Fire a fake OnChange event to initialize the connection.
	ws.OnChange(ctx, s.store)
	s.store.AddSubscriber(ctx, ws)

	ws.Stream(ctx, s.store)
	atomic.AddInt32(&s.numWebsocketConns, -1)
}

var _ store.TearDowner = WebsocketSubscriber{}
