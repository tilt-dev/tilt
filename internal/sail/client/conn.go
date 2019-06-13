package client

import (
	"context"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

// Helpers for stubbing out the network connection in sailClient
type SailDialer interface {
	DialContext(ctx context.Context, addr string, headers http.Header) (SailConn, error)
}

type defaultDialer struct{}

func (d defaultDialer) DialContext(ctx context.Context, addr string, headers http.Header) (SailConn, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, addr, headers)
	return conn, err
}

func ProvideSailDialer() SailDialer {
	return defaultDialer{}
}

type SailConn interface {
	WriteJSON(v interface{}) error
	NextReader() (int, io.Reader, error)
	Close() error
}

var _ SailConn = &websocket.Conn{}
