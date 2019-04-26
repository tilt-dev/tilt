package client

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type SailRoomConnectedAction struct {
	ViewURL string // URL to view the Sail room
	Err     error
}

func (SailRoomConnectedAction) Action() {}

type SailClient interface {
	MaybeBroadcast(st store.RStore)
	Connect(st store.RStore) error
	Teardown(ctx context.Context)
}

var _ SailClient = &sailClient{}

type sailClient struct {
	ctx      context.Context
	addr     model.SailURL
	roomer   SailRoomer
	dialer   SailDialer
	conn     SailConn
	mu       sync.Mutex
	initDone bool
}

func ProvideSailClient(ctx context.Context, addr model.SailURL, roomer SailRoomer, dialer SailDialer) *sailClient {
	return &sailClient{
		ctx:    ctx,
		addr:   addr,
		roomer: roomer,
		dialer: dialer,
	}
}

func (s *sailClient) Teardown(ctx context.Context) {
	s.disconnect()
}

func (s *sailClient) disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return
	}

	_ = s.conn.Close()
	s.conn = nil
}

func (s *sailClient) isConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

func (s *sailClient) MaybeBroadcast(st store.RStore) {
	if !s.isConnected() {
		return
	}

	state := st.RLockState()
	view := webview.StateToWebView(state)
	st.RUnlockState()

	s.broadcast(view)
}
func (s *sailClient) broadcast(view webview.View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return
	}

	err := s.conn.WriteJSON(view)
	if err != nil {
		logger.Get(s.ctx).Infof("broadcast(%s): %v", s.addr, err)
	}
}

func (s *sailClient) setConnection(conn SailConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn

	// set up socket control handling
	go func() {
		defer s.disconnect()

		for s.ctx.Err() == nil {
			// We need to read from the connection so that the websocket
			// library handles control messages, but we can otherwise discard them.
			if _, _, err := conn.NextReader(); err != nil {
				logger.Get(s.ctx).Infof("sailClient connection: %v", err)
				return
			}
		}
	}()
}

func (s *sailClient) Connect(st store.RStore) error {
	if s.addr.Empty() {
		return fmt.Errorf("tried to connect a sailClient with an empty address")
	}
	roomID, secret, err := s.roomer.NewRoom()
	if err != nil {
		st.Dispatch(SailRoomConnectedAction{Err: err})
		return err
	}
	logger.Get(s.ctx).Infof("new room %s with secret %s\n", roomID, secret)

	err = s.shareToRoom(roomID, secret)
	if err != nil {
		st.Dispatch(SailRoomConnectedAction{Err: err})
		return err
	}

	// Send back URL to surface to user for sharing
	viewUrl := s.addr.Http()
	viewUrl.Path = fmt.Sprintf("/view/%s", roomID)
	st.Dispatch(SailRoomConnectedAction{ViewURL: viewUrl.String()})

	return nil
}

func (s *sailClient) shareToRoom(roomID model.RoomID, secret string) error {
	header := make(http.Header)
	header.Add("Origin", s.addr.Ws().String())
	header.Add(model.SailSecretKey, secret)

	connectURL := s.addr
	connectURL.Path = "/share"
	connectURL = connectURL.WithQueryParam(model.SailRoomIDKey, string(roomID))

	conn, err := s.dialer.DialContext(s.ctx, connectURL.Ws().String(), header)
	if err != nil {
		return err
	}
	s.setConnection(conn)
	return nil
}

var _ store.SubscriberLifecycle = &sailClient{}
