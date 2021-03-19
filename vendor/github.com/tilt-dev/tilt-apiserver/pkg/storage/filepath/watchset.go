package filepath

import (
	"sync"

	"k8s.io/apimachinery/pkg/watch"
)

// Keeps track of which watches need to be notified
type WatchSet struct {
	mu      sync.RWMutex
	nodes   map[int]*watchNode
	counter int
}

func NewWatchSet() *WatchSet {
	return &WatchSet{
		nodes: make(map[int]*watchNode, 10),
	}
}

// Creates a new watch with a unique id, but
// does not start sending events to it until start() is called.
func (s *WatchSet) newWatch() *watchNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	return &watchNode{
		id: s.counter,
		s:  s,
		ch: make(chan watch.Event, 10),
	}
}

// Start sending events to this watch.
func (s *WatchSet) start(w *watchNode) {
	s.mu.Lock()
	s.nodes[w.id] = w
	s.mu.Unlock()
}

func (s *WatchSet) notifyWatchers(ev watch.Event) {
	s.mu.RLock()
	for _, w := range s.nodes {
		w.ch <- ev
	}
	s.mu.RUnlock()
}

type watchNode struct {
	s  *WatchSet
	id int
	ch chan watch.Event
}

func (w *watchNode) Stop() {
	w.s.mu.Lock()
	delete(w.s.nodes, w.id)
	w.s.mu.Unlock()
}

func (w *watchNode) ResultChan() <-chan watch.Event {
	return w.ch
}
