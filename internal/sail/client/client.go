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

type SailNewRoomAction struct{}

func (SailNewRoomAction) Action() {}

type SailRoomConnectedAction struct {
	ViewURL string // URL to view the Sail room
	Err     error
}

func (SailRoomConnectedAction) Action() {}

type SailClient interface {
	store.Subscriber

	NewRoom(ctx context.Context, st store.RStore) error
}

var _ SailClient = &sailClient{}

type sailClient struct {
	addr   model.SailURL
	roomer SailRoomer
	dialer SailDialer
	conn   SailConn
	mu     sync.Mutex

	roomInfo model.SailRoomInfo // Info for room this client is talking to
}

func (s *sailClient) RoomInfo() model.SailRoomInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.roomInfo
}

func ProvideSailClient(addr model.SailURL, roomer SailRoomer, dialer SailDialer) *sailClient {
	return &sailClient{
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

	s.roomInfo = model.SailRoomInfo{}

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

func (s *sailClient) hasRoomInfo() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.roomInfo != model.SailRoomInfo{}
}

func (s *sailClient) OnChange(ctx context.Context, st store.RStore) {
	if !s.hasRoomInfo() {
		return
	}

	if !s.isConnected() {
		err := s.ShareToRoom(ctx, st)
		if err != nil {
			logger.Get(ctx).Infof("sailClient.ShareToRoom(%s): %v", s.addr, err)
			s.disconnect()
			return
		}
	}

	state := st.RLockState()
	view := webview.StateToWebView(state)
	st.RUnlockState()

	s.broadcast(ctx, view)
}

func (s *sailClient) broadcast(ctx context.Context, view webview.View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return
	}

	err := s.conn.WriteJSON(view)
	if err != nil {
		logger.Get(ctx).Infof("broadcast(%s): %v", s.addr, err)
	}
}

func (s *sailClient) setConnection(ctx context.Context, conn SailConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn

	// set up socket control handling
	go func() {
		defer s.disconnect()

		for ctx.Err() == nil {
			// We need to read from the connection so that the websocket
			// library handles control messages, but we can otherwise discard them.
			if _, _, err := conn.NextReader(); err != nil {
				logger.Get(ctx).Infof("sailClient connection: %v", err)
				return
			}
		}
	}()
}

func (s *sailClient) NewRoom(ctx context.Context, st store.RStore) error {
	if s.addr.Empty() {
		return fmt.Errorf("tried to connect a sailClient with an empty address")
	}
	roomInfo, err := s.roomer.NewRoom(ctx)
	if err != nil {
		return err
	}

	// Attach room info to sailClient so we can connect to it later
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roomInfo = roomInfo

	st.Dispatch(SailNewRoomAction{})
	return nil
}

func (s *sailClient) ShareToRoom(ctx context.Context, st store.RStore) error {
	roomInfo := s.RoomInfo()
	err := s.shareToRoom(ctx, roomInfo.RoomID, roomInfo.Secret)
	if err != nil {
		st.Dispatch(SailRoomConnectedAction{Err: err})
		return err
	}

	// Send back URL to surface to user for sharing
	viewUrl := s.addr.Http()
	viewUrl.Path = fmt.Sprintf("/view/%s", roomInfo.RoomID)
	st.Dispatch(SailRoomConnectedAction{ViewURL: viewUrl.String()})

	return nil
}

func (s *sailClient) shareToRoom(ctx context.Context, roomID model.RoomID, secret string) error {
	header := make(http.Header)
	header.Add("Origin", s.addr.Ws().String())
	header.Add(model.SailSecretKey, secret)

	connectURL := s.addr
	connectURL.Path = "/share"
	connectURL = connectURL.WithQueryParam(model.SailRoomIDKey, string(roomID))

	conn, err := s.dialer.DialContext(ctx, connectURL.Ws().String(), header)
	if err != nil {
		return err
	}
	s.setConnection(ctx, conn)
	return nil
}

var _ store.SubscriberLifecycle = &sailClient{}
