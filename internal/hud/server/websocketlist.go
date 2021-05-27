package server

import (
	"sync"
)

type WebsocketList struct {
	items []*WebsocketSubscriber
	mu    sync.RWMutex
}

func NewWebsocketList() *WebsocketList {
	return &WebsocketList{}
}

func (l *WebsocketList) Add(w *WebsocketSubscriber) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = append(l.items, w)
}

func (l *WebsocketList) Remove(w *WebsocketSubscriber) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, item := range l.items {
		if item == w {
			l.items = append(l.items[:i], l.items[i+1:]...)
			return
		}
	}
}

// Operate on all websockets in the list.
//
// While the ForEach is running, the list may not be modified.
//
// In the future, it might make sense allow modification of the list while the
// foreach runs, but then we'd need additional synchronization to make sure
// we don't get websocket send() after removal.
func (l *WebsocketList) ForEach(f func(w *WebsocketSubscriber)) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, item := range l.items {
		f(item)
	}
}
