package server

import (
	"context"
	"io"
	"log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type RoomID string

// A room where messages from a source are broadcast to all the followers.
type Room struct {
	// Immutable data
	id     RoomID
	secret string // used to authorize attempts to share to this room
	source SourceConn
	addFan chan AddFanAction
	fanOut chan FanOutAction

	// Mutable data, only read/written in the action loop.
	fans []FanConn
}

// A websocket that we only read messages from
type SourceConn interface {
	ReadMessage() (int, []byte, error)
	Close() error
}

var _ SourceConn = &websocket.Conn{}

// A websocket that we only write messages to
type FanConn interface {
	WriteMessage(int, []byte) error
	NextReader() (int, io.Reader, error)
	Close() error
}

var _ FanConn = &websocket.Conn{}

type AddFanAction struct {
	fan FanConn
}

type FanOutAction struct {
	messageType int
	data        []byte
}

func NewRoom() *Room {
	return &Room{
		id:     RoomID(uuid.New().String()),
		secret: uuid.New().String(),
		addFan: make(chan AddFanAction, 0),
	}
}

// Add a fan that consumes messages from the source.
// Calling AddFan() after Close() will error.
func (r *Room) AddFan(ctx context.Context, conn FanConn) {
	r.addFan <- AddFanAction{fan: conn}

	go func() {
		for ctx.Err() == nil {
			// We need to read from the connection so that the websocket
			// library handles control messages, but we can otherwise discard them.
			if _, _, err := conn.NextReader(); err != nil {
				// TODO(nick): Remove this fan from the room.
				log.Printf("streamFan: %v", err)
				return
			}
		}
	}()
}

// Only close the room when we know AddFan can't be called.
func (r *Room) Close() {
	close(r.addFan)
}

// Receive messages from the source websocket and put them through the state loop.
func (r *Room) ConsumeSource(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	fanOut := make(chan FanOutAction, 0)

	go func() {
		// Shutdown everything if the source shuts down
		defer close(fanOut)
		defer cancel()

		for ctx.Err() == nil {
			messageType, data, err := r.source.ReadMessage()
			if err != nil && context.Canceled != err {
				log.Printf("ConsumeSource: %v", err)
				return
			}

			fanOut <- FanOutAction{messageType: messageType, data: data}
		}
	}()

	for {
		select {
		case <-ctx.Done():

			for _, fan := range r.fans {
				_ = fan.Close()
			}
			_ = r.source.Close()

			// Consume all the fan-out messages
			for _ = range fanOut {
			}

			return ctx.Err()
		case action := <-r.addFan:
			r.fans = append(r.fans, action.fan)
		case action := <-fanOut:
			for _, fan := range r.fans {
				err := fan.WriteMessage(action.messageType, action.data)
				if err != nil {
					log.Printf("Room Fan-out: %v", err)
				}
			}
		}
	}
}
