package engine

import (
	"path/filepath"
	"sync"

	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/watch"
	"github.com/windmilleng/tilt/pkg/logger"
)

type fakeMultiWatcher struct {
	events chan watch.FileEvent
	errors chan error

	mu         sync.Mutex
	watchers   []*fakeWatcher
	subs       []chan watch.FileEvent
	subsErrors []chan error
}

func newFakeMultiWatcher() *fakeMultiWatcher {
	r := &fakeMultiWatcher{events: make(chan watch.FileEvent, 20), errors: make(chan error, 20)}
	go r.loop()
	return r
}

func (w *fakeMultiWatcher) newSub(paths []string, ignore watch.PathMatcher, _ logger.Logger) (watch.Notify, error) {
	subCh := make(chan watch.FileEvent)
	errorCh := make(chan error)
	w.mu.Lock()
	defer w.mu.Unlock()

	watcher := newFakeWatcher(subCh, errorCh, paths, ignore)
	w.watchers = append(w.watchers, watcher)
	w.subs = append(w.subs, subCh)
	w.subsErrors = append(w.subsErrors, errorCh)
	return watcher, nil
}

func (w *fakeMultiWatcher) getSubs() []chan watch.FileEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]chan watch.FileEvent{}, w.subs...)
}

func (w *fakeMultiWatcher) getSubErrors() []chan error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]chan error{}, w.subsErrors...)
}

func (w *fakeMultiWatcher) loop() {
	defer func() {
		for _, sub := range w.getSubs() {
			close(sub)
		}
	}()

	defer func() {
		for _, sub := range w.getSubErrors() {
			close(sub)
		}
	}()

	for {
		select {
		case e, ok := <-w.events:
			if !ok {
				return
			}
			for _, watcher := range w.watchers {
				if watcher.matches(e.Path()) {
					watcher.inboundCh <- e
				}
			}
		case e, ok := <-w.errors:
			if !ok {
				return
			}
			for _, sub := range w.getSubErrors() {
				sub <- e
			}
		}
	}
}

type fakeWatcher struct {
	inboundCh  chan watch.FileEvent
	outboundCh chan watch.FileEvent
	errorCh    chan error

	paths  []string
	ignore watch.PathMatcher
}

func newFakeWatcher(inboundCh chan watch.FileEvent, errorCh chan error, paths []string, ignore watch.PathMatcher) *fakeWatcher {
	for i, path := range paths {
		paths[i], _ = filepath.Abs(path)
	}

	return &fakeWatcher{
		inboundCh:  inboundCh,
		outboundCh: make(chan watch.FileEvent, 20),
		errorCh:    errorCh,
		paths:      paths,
		ignore:     ignore,
	}
}

func (w *fakeWatcher) matches(path string) bool {
	ignore, _ := w.ignore.Matches(path)
	if ignore {
		return false
	}

	for _, watched := range w.paths {
		if ospath.IsChild(watched, path) {
			return true
		}
	}
	return false
}

func (w *fakeWatcher) Start() error {
	go w.loop()
	return nil
}

func (w *fakeWatcher) Close() error {
	return nil
}

func (w *fakeWatcher) Errors() chan error {
	return w.errorCh
}

func (w *fakeWatcher) Events() chan watch.FileEvent {
	return w.outboundCh
}

func (w *fakeWatcher) loop() {
	var q []watch.FileEvent
	for {
		if len(q) == 0 {
			e, ok := <-w.inboundCh
			if !ok {
				close(w.outboundCh)
				return
			}
			q = append(q, e)
		} else {
			e := q[0]
			w.outboundCh <- e
			q = q[1:]
		}
	}
}

var _ watch.Notify = &fakeWatcher{}
