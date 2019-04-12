package server

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type RoomID string

// A room where messages from a source are broadcast to all the followers.
type Room struct {
	// Immutable data
	id     RoomID
	source SourceConn
}

// A websocket that we only read messages from
type SourceConn interface {
	ReadMessage() (int, []byte, error)
	Close() error
}

var _ SourceConn = &websocket.Conn{}

func NewRoom(conn SourceConn) *Room {
	return &Room{
		id:     RoomID(uuid.New().String()),
		source: conn,
	}
}

// Receive messages from the source websocket and put them through the state loop.
func (r *Room) ConsumeSource(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		// Shutdown everything if the source shuts down
		defer cancel()

		for ctx.Err() == nil {
			_, data, err := r.source.ReadMessage()
			if err != nil {
				log.Printf("ConsumeSource: %v", err)
				return
			}

			log.Printf("Received new message: %s", string(data))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			_ = r.source.Close()
			return ctx.Err()
		}
	}
}
