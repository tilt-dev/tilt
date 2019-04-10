package client

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type SailClient struct {
	addr     model.SailURL
	conn     *websocket.Conn
	mu       sync.Mutex
	initDone bool
}

func ProvideSailClient(addr model.SailURL) *SailClient {
	return &SailClient{addr: addr}
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

func (s *SailClient) setConnection(ctx context.Context, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn

	// set up socket control handling
	go func() {
		defer func() {
			s.disconnect()
		}()

		for ctx.Err() != nil {
			// We need to read from the connection so that the websocket
			// library handles control messages, but we can otherwise discard them.
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()
}

func (s *SailClient) Connect(ctx context.Context) error {
	header := make(http.Header)
	header.Add("Origin", s.addr.String())

	connectURL := s.addr
	connectURL.Path = "/share"
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, connectURL.String(), header)
	if err != nil {
		return err
	}
	s.setConnection(ctx, conn)
	return nil
}

func (s *SailClient) OnChange(ctx context.Context, st store.RStore) {
	if s.initDone {
		return
	}
	defer func() {
		s.initDone = true
	}()

	if s.addr.Empty() {
		return
	}

	// TODO(nick): To get an end-to-end connection working, we're just
	// going to connect to the Sail server on startup. Eventually this
	// should be changed to connect on user action.
	err := s.Connect(ctx)
	if err != nil {
		st.Dispatch(store.NewErrorAction(errors.Wrap(err, "SailClient")))
	}
	s.initDone = true
}

var _ store.SubscriberLifecycle = &SailClient{}
