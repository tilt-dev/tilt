package watch

import (
	"path/filepath"
	"sync"

	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type FakeMultiWatcher struct {
	Events chan FileEvent
	Errors chan error

	mu         sync.Mutex
	watchers   []*fakeWatcher
	subs       []chan FileEvent
	subsErrors []chan error
}

func NewFakeMultiWatcher() *FakeMultiWatcher {
	r := &FakeMultiWatcher{Events: make(chan FileEvent, 20), Errors: make(chan error, 20)}
	go r.loop()
	return r
}

func (w *FakeMultiWatcher) NewSub(paths []string, ignore PathMatcher, _ logger.Logger) (Notify, error) {
	subCh := make(chan FileEvent)
	errorCh := make(chan error)
	w.mu.Lock()
	defer w.mu.Unlock()

	// note: this doesn't clean up the subs/error channels but that's not a big deal
	cleanup := func(watcher *fakeWatcher) {
		w.mu.Lock()
		defer w.mu.Unlock()
		for i, x := range w.watchers {
			if x == watcher {
				w.watchers = append(w.watchers[:i], w.watchers[i+1:]...)
			}
		}
	}

	watcher := newFakeWatcher(subCh, errorCh, paths, ignore, cleanup)

	w.watchers = append(w.watchers, watcher)
	w.subs = append(w.subs, subCh)
	w.subsErrors = append(w.subsErrors, errorCh)
	return watcher, nil
}

func (w *FakeMultiWatcher) AllWatchPaths() []string {
	var watchPaths []string
	for _, watcher := range w.watchers {
		watchPaths = append(watchPaths, watcher.paths...)
	}
	return watchPaths
}

func (w *FakeMultiWatcher) IgnorePatternMatches(path string) (bool, error) {
	for _, watcher := range w.watchers {
		m, err := watcher.ignore.Matches(path)
		if err != nil {
			return false, err
		}
		if m {
			return true, nil
		}
	}
	return false, nil
}

func (w *FakeMultiWatcher) getSubs() []chan FileEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]chan FileEvent{}, w.subs...)
}

func (w *FakeMultiWatcher) getSubErrors() []chan error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]chan error{}, w.subsErrors...)
}

func (w *FakeMultiWatcher) loop() {
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
		case e, ok := <-w.Events:
			if !ok {
				return
			}
			for _, watcher := range w.watchers {
				if watcher.matches(e.Path()) {
					watcher.inboundCh <- e
				}
			}
		case e, ok := <-w.Errors:
			if !ok {
				return
			}
			for _, sub := range w.getSubErrors() {
				sub <- e
			}
		}
	}
}

type cleanupFunc func(w *fakeWatcher)

type fakeWatcher struct {
	inboundCh  chan FileEvent
	outboundCh chan FileEvent
	errorCh    chan error
	cleanup    cleanupFunc

	paths  []string
	ignore PathMatcher
}

func newFakeWatcher(inboundCh chan FileEvent, errorCh chan error, paths []string, ignore PathMatcher, cleanup cleanupFunc) *fakeWatcher {
	for i, path := range paths {
		paths[i], _ = filepath.Abs(path)
	}

	return &fakeWatcher{
		inboundCh:  inboundCh,
		outboundCh: make(chan FileEvent, 20),
		errorCh:    errorCh,
		cleanup:    cleanup,
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
	if w.cleanup != nil {
		w.cleanup(w)
		w.cleanup = nil
	}
	return nil
}

func (w *fakeWatcher) Errors() chan error {
	return w.errorCh
}

func (w *fakeWatcher) Events() chan FileEvent {
	return w.outboundCh
}

func (w *fakeWatcher) loop() {
	var q []FileEvent
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

var _ Notify = &fakeWatcher{}
