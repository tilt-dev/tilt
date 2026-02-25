package filepath

import (
	"sync"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
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
		id:       s.counter,
		s:        s,
		updateCh: make(chan watch.Event),
		outCh:    make(chan watch.Event),
		stopCh:   make(chan struct{}),
	}
}

func (s *WatchSet) notifyWatchers(ev watch.Event) {
	s.mu.RLock()
	for _, w := range s.nodes {
		w.updateCh <- ev
	}
	s.mu.RUnlock()
}

type watchNode struct {
	s        *WatchSet
	p        storage.SelectionPredicate
	id       int
	updateCh chan watch.Event
	outCh    chan watch.Event
	stopCh   chan struct{}
}

// Start sending events to this watch.
func (w *watchNode) Start(p storage.SelectionPredicate, initEventFactory func() ([]watch.Event, error)) error {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	w.s.nodes[w.id] = w
	initEvents, err := initEventFactory()
	if err != nil {
		return err
	}

	go func() {
		// When writing to outCh, we always check stopCh too
		// and drop events if we are stopping and the consumer is not ready.

		for _, e := range initEvents {
			select {
			case <-w.stopCh:
			case w.outCh <- e:
			}
		}

		for e := range w.updateCh {
			ok, err := p.Matches(e.Object)
			if err != nil {
				continue
			}

			if !ok {
				continue
			}

			select {
			case <-w.stopCh:
			case w.outCh <- e:
			}
		}
		close(w.outCh)
	}()

	return nil
}

func (w *watchNode) Stop() {
	close(w.stopCh)

	w.s.mu.Lock()
	delete(w.s.nodes, w.id)
	w.s.mu.Unlock()

	close(w.updateCh)
}

func (w *watchNode) ResultChan() <-chan watch.Event {
	return w.outCh
}
