package engine

import (
	context "context"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	store "github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestOneActionDispatched(t *testing.T) {
	f := newWatchManagerFixture(t)

	state := f.store.LockMutableState()
	state.WatchMounts = true
	state.ManifestStates["blorgly"] = &store.ManifestState{
		Manifest: model.Manifest{
			Name:        "blorgly",
			ConfigFiles: []string{"/a/b/c.conf"},
		},
	}
	f.store.UnlockMutableState()

	f.fswm.OnChange(f.ctx, f.store)

	go func() {
		f.notify.events <- watch.FileEvent{Path: "/a/b/c.conf"}
	}()

	f.ConsumeFSEventsUntil("/a/b/c.conf")
}

func TestNoChange(t *testing.T) {
	f := newWatchManagerFixture(t)

	state := f.store.LockMutableState()
	state.WatchMounts = true
	state.ManifestStates["blorgly"] = &store.ManifestState{
		Manifest: model.Manifest{
			Name:        "blorgly",
			ConfigFiles: []string{"/a/b/c.conf"},
		},
	}
	f.store.UnlockMutableState()

	f.fswm.OnChange(f.ctx, f.store)
	f.fswm.OnChange(f.ctx, f.store)

	go func() {
		f.notify.events <- watch.FileEvent{Path: "/a/b/c.conf"}
	}()

	f.ConsumeFSEventsUntil("/a/b/c.conf")
}

// func TestMultipleManifestsEvents(t *testing.T) {
// 	f := newWatchManagerFixture(t)

// 	state := f.store.LockMutableState()
// 	state.WatchMounts = true
// 	state.ManifestStates["blorgly"] = &store.ManifestState{
// 		Manifest: model.Manifest{
// 			Name:        "blorgly",
// 			ConfigFiles: []string{"/a/b/c.conf"},
// 		},
// 	}
// 	state.ManifestStates["server"] = &store.ManifestState{
// 		Manifest: model.Manifest{
// 			Name:        "server",
// 			ConfigFiles: []string{"/b/c/d.conf"},
// 		},
// 	}
// 	f.store.UnlockMutableState()

// 	f.fswm.OnChange(f.ctx, f.store)

// 	go func() {
// 		f.notify.events <- watch.FileEvent{Path: "/a/b/c.conf"}
// 	}()
// }

type watchManagerFixture struct {
	t      *testing.T
	ctx    context.Context
	cancel func()
	out    *bufsync.ThreadSafeBuffer
	store  *store.Store
	fswm   *WatchManager
	notify *fakeNotify
}

func newWatchManagerFixture(t *testing.T) *watchManagerFixture {
	st := store.NewStore()

	out := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)
	ctx = output.WithOutputter(ctx, output.NewOutputter(l))

	notify := newFakeNotify()

	f := &watchManagerFixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		out:    out,
		store:  st,
		notify: notify,
	}

	fswm := NewWatchManager(f.provideFakeFsWatcher)
	f.fswm = fswm

	return f
}

func (f *watchManagerFixture) ConsumeFSEventsUntil(expectedPath string) {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	pathsSeen := []string{}

	for {
		select {
		case <-ctx.Done():
			f.t.Fatalf("Timeout. Collected paths: %v", pathsSeen)
		case action := <-f.store.Actions():
			fileChange, ok := action.(manifestFilesChangedAction)
			if !ok {
				f.t.Errorf("Expected action type manifestFilesChangedAction. Actual: %T", action)
			}
			if fileChange.files[0] != expectedPath {
				pathsSeen = append(pathsSeen, fileChange.files...)
				continue
			}

			// we're done!
			return
		}
	}
}

func (f *watchManagerFixture) assertNoActions() {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case action := <-f.store.Actions():
			f.t.Errorf("Expected no actions but got %v", action)
		}
	}
}

func (f *watchManagerFixture) provideFakeFsWatcher() (watch.Notify, error) {
	return f.notify, nil
}
