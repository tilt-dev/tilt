package engine

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestWatchesReappliedOnDockerIgnoreChange(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	setManifestTarget := func(target model.DockerComposeTarget) {
		m := model.Manifest{Name: "foo"}.WithDeployTarget(target)
		mt := store.ManifestTarget{Manifest: m}
		state := f.store.LockMutableStateForTesting()
		state.UpsertManifestTarget(&mt)
		state.WatchMounts = true
		f.store.UnlockMutableState()
		f.wm.OnChange(f.ctx, f.store)
	}

	target := model.DockerComposeTarget{Name: "foo"}
	target.Mounts = []model.Mount{{LocalPath: f.Path()}}
	setManifestTarget(target.WithIgnoredLocalDirectories([]string{f.JoinPath("bar")}))

	setManifestTarget(target)

	f.ChangeFile(t, "bar")
	f.ChangeFile(t, "stop")

	actions, err := f.ReadActionsUntil("stop")
	if !assert.NoError(t, err) {
		return
	}

	var paths []string
	for _, a := range actions {
		if !assert.Equal(t, 1, len(a.files)) {
			return
		}
		paths = append(paths, a.files[0])
	}

	// not asserting exact contents because we can end up with duplicates since the old watch loop isn't stopped
	// until after the new watch loop is started
	assert.Contains(t, paths, "bar")
}

type wmFixture struct {
	*tempdir.TempDirFixture
	ctx              context.Context
	cancel           func()
	actions          chan store.Action
	store            *store.Store
	wm               *WatchManager
	fakeMultiWatcher *fakeMultiWatcher
	fakeTimerMaker   fakeTimerMaker
}

func newWMFixture(t *testing.T) *wmFixture {
	f := tempdir.NewTempDirFixture(t)
	actions := make(chan store.Action, 20)
	reducer := store.Reducer(func(ctx context.Context, s *store.EngineState, action store.Action) {
		select {
		case actions <- action:
		case <-time.After(100 * time.Millisecond):
			panic("would block when writing to action chan. perhaps too many actions or too small a buffer")
		}
	})
	st := store.NewStore(reducer, false)
	timerMaker := makeFakeTimerMaker(t)
	fakeMultiWatcher := newFakeMultiWatcher()
	wm := NewWatchManager(fakeMultiWatcher.newSub, timerMaker.maker())

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)
	go func() {
		err := st.Loop(ctx)
		if err != nil && err != context.Canceled {
			panic(fmt.Sprintf("st.Loop exited with error: %+v", err))
		}
	}()

	return &wmFixture{
		TempDirFixture:   f,
		ctx:              ctx,
		cancel:           cancel,
		actions:          actions,
		store:            st,
		wm:               wm,
		fakeMultiWatcher: fakeMultiWatcher,
		fakeTimerMaker:   timerMaker,
	}
}

func (f *wmFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
}

func (f *wmFixture) ChangeFile(t *testing.T, path string) {
	select {
	case f.fakeMultiWatcher.events <- watch.FileEvent{Path: f.JoinPath(path)}:
	default:
		t.Fatal("emitting a FileEvent would block. Perhaps there are too many events or the buffer size is too small.")
	}
}

func (f *wmFixture) ReadActionsUntil(lastFile string) ([]targetFilesChangedAction, error) {
	var actions []targetFilesChangedAction
	var done = false
	for !done {
		var tfca targetFilesChangedAction
		select {
		case a := <-f.actions:
			tfca = a.(targetFilesChangedAction)

			// 1. unpack to one file per action, for deterministic inspection
			// 2. make paths relative to cwd
			for _, absPath := range tfca.files {
				p := absPath
				if relPath, ok := ospath.Child(f.Path(), absPath); ok {
					p = relPath
				}
				actions = append(actions, targetFilesChangedAction{tfca.targetID, []string{p}})
				if p == lastFile {
					done = true
					break
				}
			}
		case <-time.After(1 * time.Second):
			return nil, fmt.Errorf("timed out waiting for actions. received so far: %v", actions)
		}
	}
	return actions, nil
}
