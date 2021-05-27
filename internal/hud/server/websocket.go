package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	EnableCompression: true,
}

type WebsocketSubscriber struct {
	ctx        context.Context
	conn       WebsocketConn
	streamDone chan bool

	clientCheckpoint logstore.Checkpoint
}

type WebsocketConn interface {
	NextReader() (int, io.Reader, error)
	Close() error
	NextWriter(messageType int) (io.WriteCloser, error)
}

var _ WebsocketConn = &websocket.Conn{}

func NewWebsocketSubscriber(ctx context.Context, conn WebsocketConn) *WebsocketSubscriber {
	return &WebsocketSubscriber{
		ctx:        ctx,
		conn:       conn,
		streamDone: make(chan bool),
	}
}

func (ws *WebsocketSubscriber) TearDown(ctx context.Context) {
	_ = ws.conn.Close()
}

// Should be called exactly once. Consumes messages until the socket closes.
func (ws *WebsocketSubscriber) Stream(ctx context.Context, store *store.Store) {
	go func() {
		// No-op consumption of all control messages, as recommended here:
		// https://godoc.org/github.com/gorilla/websocket#hdr-Control_Messages
		conn := ws.conn
		for {
			_, _, err := conn.NextReader()
			if err != nil {
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

func (ws *WebsocketSubscriber) InitializeStream(ctx context.Context, s store.RStore) {
	ws.onChangeHelper(ctx, s, 0, nil)
}

func (ws *WebsocketSubscriber) OnChange(ctx context.Context, s store.RStore, summary store.ChangeSummary) {
	ws.onChangeHelper(ctx, s, ws.clientCheckpoint, &summary)
}

func (ws *WebsocketSubscriber) onChangeHelper(ctx context.Context, s store.RStore, checkpoint logstore.Checkpoint, summary *store.ChangeSummary) {
	state := s.RLockState()
	view, err := webview.ChangeSummaryToProtoView(state, checkpoint, summary)
	if view.UiSession != nil && view.UiSession.Status.NeedsAnalyticsNudge && !state.AnalyticsNudgeSurfaced {
		// If we're showing the nudge and no one's told the engine
		// state about it yet... tell the engine state.
		s.Dispatch(store.AnalyticsNudgeSurfacedAction{})
	}

	s.RUnlockState()
	if err != nil {
		logger.Get(ctx).Infof("error converting view to proto for websocket: %v", err)
		return
	}

	if view.LogList != nil {
		ws.clientCheckpoint = logstore.Checkpoint(view.LogList.ToCheckpoint)
	}

	ws.sendView(ctx, s, view)

	// A simple throttle -- don't call ws.OnChange too many times in quick succession,
	//     it eats up a lot of CPU/allocates a lot of memory.
	// This is safe b/c (as long as we're not holding a lock on the state, which
	//     at this point in the code, we're not) the only thing ws.OnChange blocks
	//     is subsequent ws.OnChange calls.
	//
	// In future, we can maybe solve this problem more elegantly by replacing our
	//     JSON marshaling with jsoniter (though changing json marshalers is
	//     always fraught with peril).
	time.Sleep(time.Millisecond * 100)
}

// Sends the view to the websocket.
func (ws *WebsocketSubscriber) sendView(ctx context.Context, s store.RStore, view *proto_webview.View) {

	jsEncoder := &runtime.JSONPb{}
	w, err := ws.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		logger.Get(ctx).Verbosef("getting writer: %v", err)
		return
	}
	defer func() {
		err := w.Close()
		if err != nil {
			logger.Get(ctx).Verbosef("error closing websocket writer: %v", err)
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
	ws := NewWebsocketSubscriber(s.ctx, conn)

	// Fire a fake OnChange event to initialize the connection.
	ws.InitializeStream(s.ctx, s.store)
	_ = s.store.AddSubscriber(s.ctx, ws)

	ws.Stream(s.ctx, s.store)
	atomic.AddInt32(&s.numWebsocketConns, -1)
}

var _ store.TearDowner = &WebsocketSubscriber{}
