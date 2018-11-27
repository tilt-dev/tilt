package engine

import (
	"sync"

	"github.com/windmilleng/tilt/internal/watch"
)

type fakeMetaWatcher struct {
	events chan watch.FileEvent

	mu   sync.Mutex
	subs []chan watch.FileEvent
}

func newFakeMetaWatcher() *fakeMetaWatcher {
	r := &fakeMetaWatcher{events: make(chan watch.FileEvent)}
	go r.loop()
	return r
}

func (w *fakeMetaWatcher) newSub() (watch.Notify, error) {
	subCh := make(chan watch.FileEvent)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.subs = append(w.subs, subCh)
	return newFakeWatcher(subCh), nil
}

func (w *fakeMetaWatcher) getSubs() []chan watch.FileEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.subs
}

func (w *fakeMetaWatcher) loop() {
	for {
		select {
		case e, ok := <-w.events:
			if !ok {
				for _, sub := range w.getSubs() {
					close(sub)
				}
				return
			}
			for _, sub := range w.getSubs() {
				sub <- e
			}
		}
	}
}

type fakeWatcher struct {
	inboundCh  chan watch.FileEvent
	outboundCh chan watch.FileEvent
}

func newFakeWatcher(inboundCh chan watch.FileEvent) *fakeWatcher {
	r := &fakeWatcher{inboundCh: inboundCh, outboundCh: make(chan watch.FileEvent)}
	go r.loop()

	return r
}

func (w *fakeWatcher) Add(name string) error {
	return nil
}

func (w *fakeWatcher) Close() error {
	return nil
}

func (w *fakeWatcher) Errors() chan error {
	return nil
}

func (w *fakeWatcher) Events() chan watch.FileEvent {
	return w.outboundCh
}

func (w *fakeWatcher) loop() {
	var q []watch.FileEvent
	for {
		var outboundCh chan watch.FileEvent
		var outboundE watch.FileEvent
		if len(q) > 0 {
			outboundCh, outboundE = w.outboundCh, q[0]
		}

		select {
		case e, ok := <-w.inboundCh:
			if !ok {
				close(w.outboundCh)
				return
			}
			q = append(q, e)
		case outboundCh <- outboundE:
			q = q[1:]
		}
	}
}

var _ watch.Notify = &fakeWatcher{}
