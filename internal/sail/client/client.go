package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/types"
	"github.com/windmilleng/tilt/internal/store"
)

type SailClient struct {
	addr     model.SailURL
	dialer   SailDialer
	conn     SailConn
	mu       sync.Mutex
	initDone bool
}

func ProvideSailClient(dialer SailDialer, addr model.SailURL) *SailClient {
	return &SailClient{
		addr:   addr,
		dialer: dialer,
	}
}

func (s *SailClient) Teardown(ctx context.Context) {
	s.disconnect()
}

func (s *SailClient) disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return
	}

	_ = s.conn.Close()
	s.conn = nil
}

func (s *SailClient) isConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

func (s *SailClient) broadcast(ctx context.Context, view webview.View) {
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

func (s *SailClient) setConnection(ctx context.Context, conn SailConn) {
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
				logger.Get(ctx).Infof("SailClient connection: %v", err)
				return
			}
		}
	}()
}

func (s *SailClient) Connect(ctx context.Context) error {
	roomID, secret, err := s.newRoom(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("new room %s with secret %s", roomID, secret)
	return nil
}

func (s *SailClient) newRoom(ctx context.Context) (roomID, secret string, err error) {
	addr := s.addr.Http()
	header := make(http.Header)
	header.Add("Origin", addr.String())

	connectURL := addr
	connectURL.Path = "/new_room"
	resp, err := http.Get(connectURL.String())
	if err != nil {
		return "", "", errors.Wrapf(err, "GET %s", connectURL.String())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.Wrap(err, "reading /new_room response")
	}

	var roomInfo types.RoomInfo
	err = json.Unmarshal(body, &roomInfo)
	if err != nil {
		return "", "", errors.Wrapf(err, "unmarshaling json: %s", string(body))
	}

	return roomInfo.RoomID, roomInfo.Secret, nil
}

func (s *SailClient) shareToRoom(ctx context.Context) error {
	header := make(http.Header)
	header.Add("Origin", s.addr.Ws().String())

	connectURL := s.addr
	connectURL.Path = "/share"
	conn, err := s.dialer.DialContext(ctx, connectURL.Ws().String(), header)
	if err != nil {
		return err
	}
	s.setConnection(ctx, conn)
	return nil
}

func (s *SailClient) init(ctx context.Context) error {
	if s.addr.Empty() {
		return nil
	}

	return s.Connect(ctx)
}

func (s *SailClient) OnChange(ctx context.Context, st store.RStore) {
	if !s.initDone {
		s.initDone = true

		// TODO(nick): To get an end-to-end connection working, we're just
		// going to connect to the Sail server on startup. Eventually this
		// should be changed to connect on user action.
		err := s.init(ctx)
		if err != nil {
			st.Dispatch(store.NewErrorAction(errors.Wrap(err, "SailClient")))
		}
	}

	if !s.isConnected() {
		return
	}

	state := st.RLockState()
	view := server.StateToWebView(state)
	st.RUnlockState()

	s.broadcast(ctx, view)
}

var _ store.SubscriberLifecycle = &SailClient{}
