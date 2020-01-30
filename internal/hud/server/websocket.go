package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model/logstore"
	proto_webview "github.com/windmilleng/tilt/pkg/webview"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WebsocketSubscriber struct {
	ctx        context.Context
	conn       WebsocketConn
	streamDone chan bool

	mu               sync.Mutex
	tiltStartTime    time.Time
	clientCheckpoint logstore.Checkpoint
}

type WebsocketConn interface {
	NextReader() (int, io.Reader, error)
	Close() error
	WriteJSON(v interface{}) error
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
			messageType, reader, err := conn.NextReader()
			if err != nil {
				close(ws.streamDone)
				break
			}

			if messageType == websocket.TextMessage {
				err := ws.handleClientMessage(reader)
				if err != nil {
					logger.Get(ctx).Infof("Error parsing webclient message: %v", err)
				}
			}
		}
	}()

	<-ws.streamDone

	// When we remove ourselves as a subscriber, the Store waits for any outstanding OnChange
	// events to complete, then calls TearDown.
	_ = store.RemoveSubscriber(context.Background(), ws)
}

func (ws *WebsocketSubscriber) handleClientMessage(reader io.Reader) error {
	decoder := (&runtime.JSONPb{OrigName: false}).NewDecoder(reader)
	msg := &proto_webview.AckWebsocketRequest{}
	err := decoder.Decode(msg)
	if err != nil {
		return err
	}

	return ws.updateClientCheckpoint(msg)
}

func (ws *WebsocketSubscriber) readClientCheckpoint() logstore.Checkpoint {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.clientCheckpoint
}

func (ws *WebsocketSubscriber) setTiltStartTime(t time.Time) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.tiltStartTime = t
}

func (ws *WebsocketSubscriber) updateClientCheckpoint(msg *proto_webview.AckWebsocketRequest) error {
	t, err := ptypes.Timestamp(msg.TiltStartTime)
	if err != nil {
		return err
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !t.Equal(ws.tiltStartTime) {
		ws.clientCheckpoint = 0
		return nil
	}

	ws.clientCheckpoint = logstore.Checkpoint(msg.ToCheckpoint)
	return nil
}

func (ws *WebsocketSubscriber) OnChange(ctx context.Context, s store.RStore) {
	checkpoint := ws.readClientCheckpoint()

	state := s.RLockState()
	view, err := webview.StateToProtoView(state, checkpoint)
	tiltStartTime := state.TiltStartTime
	s.RUnlockState()
	if err != nil {
		logger.Get(ctx).Infof("error converting view to proto for websocket: %v", err)
		return
	}

	ws.setTiltStartTime(tiltStartTime)

	if view.NeedsAnalyticsNudge && !state.AnalyticsNudgeSurfaced {
		// If we're showing the nudge and no one's told the engine
		// state about it yet... tell the engine state.
		s.Dispatch(store.AnalyticsNudgeSurfacedAction{})
	}

	jsEncoder := &runtime.JSONPb{OrigName: false, EmitDefaults: true}
	w, err := ws.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		logger.Get(ctx).Verbosef("getting writer: %v", err)
		return
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

	// A simple throttle -- don't call ws.OnChange too many times in quick succession,
	//     it eats up a lot of CPU/allocates a lot of memory.
	// This is safe b/c (as long as we're not holding a lock on the state, which
	//     at this point in the code, we're not) the only thing ws.OnChange blocks
	//     is subsequent ws.OnChange calls.
	//
	// In future, we can maybe solve this problem more elegantly by replacing our JSON
	//     marshaling with jsoniter (would require either working around the lack of an
	//     `EmitDefaults` option in jsoniter, or writing our own proto marshaling code).
	time.Sleep(time.Millisecond * 100)
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
	ws.OnChange(s.ctx, s.store)
	s.store.AddSubscriber(s.ctx, ws)

	ws.Stream(s.ctx, s.store)
	atomic.AddInt32(&s.numWebsocketConns, -1)
}

var _ store.TearDowner = &WebsocketSubscriber{}
