package command

import (
	"sync"

	"github.com/docker/docker/api/types/events"
	"github.com/sirupsen/logrus"
)

// EventHandler is abstract interface for user to customize
// own handle functions of each type of events
type EventHandler interface {
	Handle(action string, h func(events.Message))
	Watch(c <-chan events.Message)
}

// InitEventHandler initializes and returns an EventHandler
func InitEventHandler() EventHandler {
	return &eventHandler{handlers: make(map[string]func(events.Message))}
}

type eventHandler struct {
	handlers map[string]func(events.Message)
	mu       sync.Mutex
}

func (w *eventHandler) Handle(action string, h func(events.Message)) {
	w.mu.Lock()
	w.handlers[action] = h
	w.mu.Unlock()
}

// Watch ranges over the passed in event chan and processes the events based on the
// handlers created for a given action.
// To stop watching, close the event chan.
func (w *eventHandler) Watch(c <-chan events.Message) {
	for e := range c {
		w.mu.Lock()
		h, exists := w.handlers[e.Action]
		w.mu.Unlock()
		if !exists {
			continue
		}
		logrus.Debugf("event handler: received event: %v", e)
		go h(e)
	}
}
