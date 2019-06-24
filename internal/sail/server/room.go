package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/windmilleng/tilt/internal/model"
)

// A room where messages from a source are broadcast to all the followers.
type Room struct {
	// Immutable data
	id        model.RoomID
	secret    string // used to authorize attempts to share to this room
	source    SourceConn
	fanWG     sync.WaitGroup
	addFan    chan AddFanAction
	removeFan chan RemoveFanAction
	fanOut    chan FanOutAction
	version   model.WebVersion // version of data feeding this room (+ version of assets to serve)

	// Mutable data, only read/written in the action loop.
	fans       []FanConn
	lastFanOut FanOutAction
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

type RemoveFanAction struct {
	fan FanConn
}

type FanOutAction struct {
	messageType int
	data        []byte
}

func (a FanOutAction) Empty() bool { return a.messageType == 0 && len(a.data) == 0 }

func NewRoom(version model.WebVersion) *Room {
	return &Room{
		id:        model.RoomID(uuid.New().String()),
		secret:    uuid.New().String(),
		version:   version,
		addFan:    make(chan AddFanAction, 0),
		removeFan: make(chan RemoveFanAction, 0),
	}
}

// newRoomResponse returns json bytes containing all information about this room that we want
// to return to the caller of the /new_room endpoint
func (r *Room) newRoomResponse() ([]byte, error) {
	info := model.SailRoomInfo{
		RoomID: r.id,
		Secret: r.secret,
	}
	return json.Marshal(info)
}

// Add a fan that consumes messages from the source.
// Calling AddFan() after Close() will error.
func (r *Room) AddFan(ctx context.Context, conn FanConn) {
	r.fanWG.Add(1)
	r.addFan <- AddFanAction{fan: conn}

	go func() {
		defer func() {
			r.removeFan <- RemoveFanAction{fan: conn}
			r.fanWG.Done()
		}()

		for ctx.Err() == nil {
			// We need to read from the connection so that the websocket
			// library handles control messages, but we can otherwise discard them.
			if _, _, err := conn.NextReader(); err != nil {
				log.Printf("streamFan: %v", err)
				return
			}
		}
	}()
}

// Only close the room when we know AddFan can't be called.
func (r *Room) Close() {
	go func() {
		// Drain the channels
		for {
			select {
			case _, ok := <-r.addFan:
				if !ok {
					return
				}
			case _, ok := <-r.removeFan:
				if !ok {
					return
				}
			}
		}
	}()
	r.fanWG.Wait()
	close(r.addFan)
	close(r.removeFan)
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
			fan := action.fan
			r.fans = append(r.fans, fan)

			// Make sure that the newly connected fan has some data.
			// TODO: the more robust way to do this is for joiner to "request" an update
			// (and have the request propagate back to Tilt)
			if !r.lastFanOut.Empty() {
				err := fan.WriteMessage(r.lastFanOut.messageType, r.lastFanOut.data)
				if err != nil {
					log.Printf("MostRecentAction to new fan: %v", err)
				}
			}
		case action := <-r.removeFan:
			for i, fan := range r.fans {
				if fan == action.fan {
					r.fans = append(r.fans[:i], r.fans[i+1:]...)
					break
				}
			}
		case action := <-fanOut:
			for _, fan := range r.fans {
				err := fan.WriteMessage(action.messageType, action.data)
				if err != nil {
					log.Printf("Room Fan-out: %v", err)
				}
			}
			r.lastFanOut = action
		}
	}
}
