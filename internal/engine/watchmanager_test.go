package engine

import (
	context "context"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/model"
	store "github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
)

// TODO(dbentley): these tests aren't testing that it watches the right thing, just that it passes events up.
// Fix this.
func TestOneActionDispatched(t *testing.T) {
	f := newWatchManagerFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.ManifestStates["blorgly"] = &store.ManifestState{
		Manifest: model.Manifest{
			Name: "blorgly",
			// ConfigFiles: []string{"/a/b/c.conf"},
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
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.ManifestStates["blorgly"] = &store.ManifestState{
		Manifest: model.Manifest{
			Name: "blorgly",
			// ConfigFiles: []string{"/a/b/c.conf"},
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

// 	state := f.store.LockMutableStateForTesting()
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
	t         *testing.T
	ctx       context.Context
	cancel    func()
	store     *store.Store
	fswm      *WatchManager
	notify    *fakeNotify
	pathsSeen []string
}

func newWatchManagerFixture(t *testing.T) *watchManagerFixture {

	ctx, cancel := context.WithCancel(context.Background())
	notify := newFakeNotify()

	f := &watchManagerFixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		notify: notify,
	}

	reducer := func(ctx context.Context, state *store.EngineState, action store.Action) {
		fileChange, ok := action.(manifestFilesChangedAction)
		if !ok {
			t.Errorf("Expected action type manifestFilesChangedAction. Actual: %T", action)
		}
		f.pathsSeen = append(f.pathsSeen, fileChange.files...)
	}
	st := store.NewStore(store.Reducer(reducer), store.LogActionsFlag(false))
	f.store = st

	timerMaker := makeFakeTimerMaker(t)

	fswm := NewWatchManager(f.provideFakeFsWatcher, timerMaker.maker())
	f.fswm = fswm

	go st.Loop(ctx)
	return f
}

func (f *watchManagerFixture) ConsumeFSEventsUntil(expectedPath string) {
	start := time.Now()
	done := false
	for time.Since(start) < time.Second {
		f.store.RLockState()
		for _, path := range f.pathsSeen {
			if path == expectedPath {
				done = true
			}
		}
		f.store.RUnlockState()

		if done {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	f.t.Fatalf("Timeout. Collected paths: %v", f.pathsSeen)
}

func (f *watchManagerFixture) provideFakeFsWatcher() (watch.Notify, error) {
	return f.notify, nil
}

func (f *watchManagerFixture) TearDown() {
	f.cancel()
}
