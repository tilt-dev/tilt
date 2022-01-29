package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/protobuf/ptypes/timestamp"

	"github.com/tilt-dev/tilt/internal/hud/server/gorilla"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	// Disable compression due to safari bugs in websockets, see:
	// https://github.com/tilt-dev/tilt/issues/4746
	//
	// Though, frankly, we probably don't need compression
	// anyway, since it's not like you're using Tilt over
	// a mobile network.
	EnableCompression: false,

	// Allow the connection if either:
	//
	// 1) The client has a CSRF token, or
	// 2) The origin matches what we expect.
	//
	// Once a few releases have gone by we should remove the origin check.
	// (since we know some tilt users expect tabs to stay open
	// across releases).
	CheckOrigin: func(req *http.Request) bool {
		if websocketCSRFToken.String() == req.URL.Query().Get("csrf") {
			return true
		}

		// If the CSRF check fails, fallback to an origin check.
		return gorilla.CheckSameOrigin(req)
	},
}

type WebsocketSubscriber struct {
	ctx        context.Context
	st         store.RStore
	ctrlClient ctrlclient.Client
	mu         sync.Mutex
	conn       WebsocketConn
	initDone   chan bool
	streamDone chan bool

	tiltStartTime    *timestamp.Timestamp
	clientCheckpoint logstore.Checkpoint
}

type WebsocketConn interface {
	NextReader() (int, io.Reader, error)
	Close() error
	NextWriter(messageType int) (io.WriteCloser, error)
}

var _ WebsocketConn = &websocket.Conn{}

func NewWebsocketSubscriber(ctx context.Context, ctrlClient ctrlclient.Client, st store.RStore, conn WebsocketConn) *WebsocketSubscriber {
	return &WebsocketSubscriber{
		ctx:        ctx,
		ctrlClient: ctrlClient,
		st:         st,
		conn:       conn,
		initDone:   make(chan bool),
		streamDone: make(chan bool),
	}
}

func (ws *WebsocketSubscriber) TearDown(ctx context.Context) {
	_ = ws.conn.Close()
}

// Should be called exactly once. Consumes messages until the socket closes.
func (ws *WebsocketSubscriber) Stream(ctx context.Context) {
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer close(ws.initDone)

		// initialize the stream with a full view
		view, err := webview.CompleteView(ctx, ws.ctrlClient, ws.st)
		if err != nil {
			// not much to do
			return
		}

		ws.sendView(ctx, view)

		if view.UiSession != nil {
			ws.onSessionUpdateSent(ctx, view.UiSession)
		}
	}()

	<-ws.streamDone
}

func (ws *WebsocketSubscriber) waitForInitOrClose(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-ws.streamDone:
		return false
	case <-ws.initDone:
		return true
	}
}

func (ws *WebsocketSubscriber) OnChange(ctx context.Context, s store.RStore, summary store.ChangeSummary) error {
	// Currently, we only broadcast log changes from this OnChange handler.
	// Everything else should be handled by reconcilers from the apiserver
	if !summary.Log {
		return nil
	}

	if !ws.waitForInitOrClose(ctx) {
		return nil
	}

	view, err := webview.LogUpdate(s, ws.clientCheckpoint)
	if err != nil {
		return nil // Not much we can do on error right now.
	}

	// [-1,-1) means there are no logs
	if view.LogList.ToCheckpoint == -1 && view.LogList.FromCheckpoint == -1 {
		return nil
	}

	ws.sendView(ctx, view)

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
	return nil
}

// Sends a UISession update on the websocket.
func (ws *WebsocketSubscriber) SendUISessionUpdate(ctx context.Context, uiSession *v1alpha1.UISession) {
	if !ws.waitForInitOrClose(ctx) {
		return
	}

	ws.sendView(ctx, &proto_webview.View{
		TiltStartTime: ws.tiltStartTime,
		UiSession:     uiSession,
	})
	ws.onSessionUpdateSent(ctx, uiSession)
}

// If a session update triggered an analytics nudge, record it so that we don't
// nudge again.
func (ws *WebsocketSubscriber) onSessionUpdateSent(ctx context.Context, uiSession *v1alpha1.UISession) {
	state := ws.st.RLockState()
	surfaced := !state.AnalyticsNudgeSurfaced
	ws.st.RUnlockState()

	if uiSession != nil && uiSession.Status.NeedsAnalyticsNudge && !surfaced {
		// If we're showing the nudge and no one's told the engine
		// state about it yet... tell the engine state.
		ws.st.Dispatch(store.AnalyticsNudgeSurfacedAction{})
	}
}

// Sends a UIResource update on the websocket.
func (ws *WebsocketSubscriber) SendUIResourceUpdate(ctx context.Context, nn types.NamespacedName, uiResource *v1alpha1.UIResource) {
	if !ws.waitForInitOrClose(ctx) {
		return
	}

	if uiResource == nil {
		// If the UI resource doesn't exist, send a fake one down the
		// stream that the UI will interpret as deletion.
		now := metav1.Now()
		uiResource = &v1alpha1.UIResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:              nn.Name,
				DeletionTimestamp: &now,
			},
		}
	}

	ws.sendView(ctx, &proto_webview.View{
		TiltStartTime: ws.tiltStartTime,
		UiResources:   []*v1alpha1.UIResource{uiResource},
	})
}

// Sends a UIButton update on the websocket.
func (ws *WebsocketSubscriber) SendUIButtonUpdate(ctx context.Context, nn types.NamespacedName, uiButton *v1alpha1.UIButton) {
	if !ws.waitForInitOrClose(ctx) {
		return
	}

	if uiButton == nil {
		// If the UI button doesn't exist, send a fake one down the
		// stream that the UI will interpret as deletion.
		now := metav1.Now()
		uiButton = &v1alpha1.UIButton{
			ObjectMeta: metav1.ObjectMeta{
				Name:              nn.Name,
				DeletionTimestamp: &now,
			},
		}
	}

	ws.sendView(ctx, &proto_webview.View{
		TiltStartTime: ws.tiltStartTime,
		UiButtons:     []*v1alpha1.UIButton{uiButton},
	})
}

// Sends the view to the websocket.
func (ws *WebsocketSubscriber) sendView(ctx context.Context, view *proto_webview.View) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if view.LogList != nil {
		ws.clientCheckpoint = logstore.Checkpoint(view.LogList.ToCheckpoint)
	}

	// A little hack that initializes tiltStartTime for this websocket
	// on the first send.
	if ws.tiltStartTime == nil {
		ws.tiltStartTime = view.TiltStartTime
	}

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

	ws := NewWebsocketSubscriber(s.ctx, s.ctrlClient, s.store, conn)
	s.wsList.Add(ws)
	_ = s.store.AddSubscriber(s.ctx, ws)

	ws.Stream(s.ctx)

	// When we remove ourselves as a subscriber, the Store waits for any outstanding OnChange
	// events to complete, then calls TearDown.
	_ = s.store.RemoveSubscriber(context.Background(), ws)
	s.wsList.Remove(ws)
}

var _ store.TearDowner = &WebsocketSubscriber{}
