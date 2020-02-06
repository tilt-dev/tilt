package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/watch"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestWatchManager_IgnoredLocalDirectories(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithIgnoredLocalDirectories([]string{"bar"}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, "bar/baz")

	actions := f.Stop(t)
	f.AssertActionsNotContain(actions, "bar/baz")
}

func TestWatchManager_Dockerignore(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithDockerignores([]model.Dockerignore{{LocalPath: ".", Contents: "bar"}}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, "bar/baz")

	actions := f.Stop(t)

	f.AssertActionsNotContain(actions, "bar/baz")
}

func TestWatchManager_WatchesReappliedOnDockerComposeSyncChange(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target.WithIgnoredLocalDirectories([]string{"bar"}))
	f.SetManifestTarget(target)

	f.ChangeFile(t, "bar")

	actions := f.Stop(t)

	// not asserting exact contents because we can end up with duplicates since the old watch loop isn't stopped
	// until after the new watch loop is started
	f.AssertActionsContain(actions, "bar")
}

func TestWatchManager_WatchesReappliedOnDockerIgnoreChange(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target.WithDockerignores([]model.Dockerignore{{LocalPath: ".", Contents: "bar"}}))
	f.SetManifestTarget(target)

	f.ChangeFile(t, "bar")

	actions := f.Stop(t)

	// not asserting exact contents because we can end up with duplicates since the old watch loop isn't stopped
	// until after the new watch loop is started
	f.AssertActionsContain(actions, "bar")
}

func TestWatchManager_IgnoreTiltIgnore(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)
	f.SetTiltIgnoreContents("**/foo")

	f.ChangeFile(t, "bar/foo")

	actions := f.Stop(t)

	f.AssertActionsNotContain(actions, "bar/foo")
}

func TestWatchManager_PickUpTiltIgnoreChanges(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)
	f.SetTiltIgnoreContents("**/foo")
	f.ChangeFile(t, "bar/foo")
	f.SetTiltIgnoreContents("**foo\n!bar/baz/foo")
	f.ChangeFile(t, "bar/baz/foo")

	actions := f.Stop(t)
	f.AssertActionsNotContain(actions, "bar/foo")
	f.AssertActionsContain(actions, "bar/baz/foo")
}

type wmFixture struct {
	ctx              context.Context
	cancel           func()
	getActions       func() []store.Action
	store            *store.Store
	wm               *WatchManager
	fakeMultiWatcher *fakeMultiWatcher
	fakeTimerMaker   fakeTimerMaker
	*tempdir.TempDirFixture
}

func newWMFixture(t *testing.T) *wmFixture {
	st, getActions := store.NewStoreForTesting()
	timerMaker := makeFakeTimerMaker(t)
	fakeMultiWatcher := newFakeMultiWatcher()
	wm := NewWatchManager(fakeMultiWatcher.newSub, timerMaker.maker())

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		err := st.Loop(ctx)
		if err != nil && err != context.Canceled {
			panic(fmt.Sprintf("st.Loop exited with error: %+v", err))
		}
	}()

	f := tempdir.NewTempDirFixture(t)
	f.Chdir()

	return &wmFixture{
		ctx:              ctx,
		cancel:           cancel,
		getActions:       getActions,
		store:            st,
		wm:               wm,
		fakeMultiWatcher: fakeMultiWatcher,
		fakeTimerMaker:   timerMaker,
		TempDirFixture:   f,
	}
}

func (f *wmFixture) TearDown() {
	f.TempDirFixture.TearDown()
	f.cancel()
}

func (f *wmFixture) ChangeFile(t *testing.T, path string) {
	path, _ = filepath.Abs(path)

	select {
	case f.fakeMultiWatcher.events <- watch.NewFileEvent(path):
	default:
		t.Fatal("emitting a FileEvent would block. Perhaps there are too many events or the buffer size is too small.")
	}
}

func (f *wmFixture) AssertActionsContain(actions []targetFilesChangedAction, path string) {
	path, _ = filepath.Abs(path)
	observedPaths := targetFilesChangedActionsToPaths(actions)
	assert.Contains(f.T(), observedPaths, path)
}

func (f *wmFixture) AssertActionsNotContain(actions []targetFilesChangedAction, path string) {
	path, _ = filepath.Abs(path)
	observedPaths := targetFilesChangedActionsToPaths(actions)
	assert.NotContains(f.T(), observedPaths, path)
}

func (f *wmFixture) ReadActionsUntil(lastFile string) ([]targetFilesChangedAction, error) {
	lastFile, _ = filepath.Abs(lastFile)
	startTime := time.Now()
	timeout := time.Second
	var actions []targetFilesChangedAction
	for time.Since(startTime) < timeout {
		actions = nil
		for _, a := range f.getActions() {
			tfca, ok := a.(targetFilesChangedAction)
			if !ok {
				return nil, fmt.Errorf("expected action of type %T, got action of type %T: %v", targetFilesChangedAction{}, a, a)
			}
			// 1. unpack to one file per action, for deterministic inspection
			// 2. make paths relative to cwd
			for _, p := range tfca.files {
				actions = append(actions, newTargetFilesChangedAction(tfca.targetID, p))
				if p == lastFile {
					return actions, nil
				}
			}
		}

	}
	return nil, fmt.Errorf("timed out waiting for actions. received so far: %v", actions)
}

func (f *wmFixture) Stop(t *testing.T) []targetFilesChangedAction {
	f.ChangeFile(t, "stop")

	actions, err := f.ReadActionsUntil("stop")
	if err != nil {
		fmt.Printf("This is due to something funky in the test itself, not the code being tested.\n")
		t.Fatal(err)
	}
	return actions
}

func (f *wmFixture) SetManifestTarget(target model.DockerComposeTarget) {
	m := model.Manifest{Name: "foo"}.WithDeployTarget(target)
	mt := store.ManifestTarget{Manifest: m}
	state := f.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(&mt)
	state.WatchFiles = true
	f.store.UnlockMutableState()
	f.wm.OnChange(f.ctx, f.store)
}

func (f *wmFixture) SetTiltIgnoreContents(s string) {
	state := f.store.LockMutableStateForTesting()
	state.TiltIgnoreContents = s
	f.store.UnlockMutableState()
	f.wm.OnChange(f.ctx, f.store)
}

func targetFilesChangedActionsToPaths(actions []targetFilesChangedAction) []string {
	var paths []string
	for _, a := range actions {
		paths = append(paths, a.files...)
	}
	return paths
}
