package fswatch

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestWatchManager_basic(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, "foo.txt")

	actions := f.Stop(t)
	f.AssertActionsContain(actions, "foo.txt")
}

func TestWatchManager_disabledOnCIMode(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
	state.EngineMode = store.EngineModeCI
	f.store.UnlockMutableState()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, "foo.txt")

	store.AssertNoActionOfType(t, reflect.TypeOf(TargetFilesChangedAction{}), f.store.Actions)
}

func TestWatchManager_IgnoredLocalDirectories(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithIgnoredLocalDirectories([]string{"bar"}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, filepath.Join("bar", "baz"))

	actions := f.Stop(t)
	f.AssertActionsNotContain(actions, filepath.Join("bar", "baz"))
}

func TestWatchManager_Dockerignore(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithDockerignores([]model.Dockerignore{{LocalPath: ".", Contents: "bar"}}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, filepath.Join("bar", "baz"))

	actions := f.Stop(t)

	f.AssertActionsNotContain(actions, filepath.Join("bar", "baz"))
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

	f.ChangeFile(t, filepath.Join("bar", "foo"))

	actions := f.Stop(t)

	f.AssertActionsNotContain(actions, filepath.Join("bar", "foo"))
}

func TestWatchManager_PickUpTiltIgnoreChanges(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)
	f.SetTiltIgnoreContents("**/foo")
	f.ChangeFile(t, filepath.Join("bar", "foo"))
	f.SetTiltIgnoreContents("**foo\n!bar/baz/foo")
	f.ChangeFile(t, filepath.Join("bar", "baz", "foo"))

	actions := f.Stop(t)
	f.AssertActionsNotContain(actions, filepath.Join("bar", "foo"))
	f.AssertActionsContain(actions, filepath.Join("bar", "baz", "foo"))
}

type wmFixture struct {
	ctx              context.Context
	cancel           func()
	store            *store.TestingStore
	wm               *WatchManager
	fakeMultiWatcher *FakeMultiWatcher
	fakeTimerMaker   FakeTimerMaker
	*tempdir.TempDirFixture
}

func newWMFixture(t *testing.T) *wmFixture {
	st := store.NewTestingStore()
	timerMaker := MakeFakeTimerMaker(t)
	fakeMultiWatcher := NewFakeMultiWatcher()
	wm := NewWatchManager(fakeMultiWatcher.NewSub, timerMaker.Maker())

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	f := tempdir.NewTempDirFixture(t)
	f.Chdir()

	return &wmFixture{
		ctx:              ctx,
		cancel:           cancel,
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
	f.store.AssertNoErrorActions(f.T())
}

func (f *wmFixture) ChangeFile(t *testing.T, path string) {
	path, _ = filepath.Abs(path)

	select {
	case f.fakeMultiWatcher.Events <- watch.NewFileEvent(path):
	default:
		t.Fatal("emitting a FileEvent would block. Perhaps there are too many events or the buffer size is too small.")
	}
}

func (f *wmFixture) AssertActionsContain(actions []TargetFilesChangedAction, path string) {
	path, _ = filepath.Abs(path)
	observedPaths := targetFilesChangedActionsToPaths(actions)
	assert.Contains(f.T(), observedPaths, path)
}

func (f *wmFixture) AssertActionsNotContain(actions []TargetFilesChangedAction, path string) {
	path, _ = filepath.Abs(path)
	observedPaths := targetFilesChangedActionsToPaths(actions)
	assert.NotContains(f.T(), observedPaths, path)
}

func (f *wmFixture) ReadActionsUntil(lastFile string) ([]TargetFilesChangedAction, error) {
	lastFile, _ = filepath.Abs(lastFile)
	startTime := time.Now()
	timeout := time.Second
	var actions []TargetFilesChangedAction
	for time.Since(startTime) < timeout {
		actions = nil
		for _, a := range f.store.Actions() {
			tfca, ok := a.(TargetFilesChangedAction)
			if !ok {
				return nil, fmt.Errorf("expected action of type %T, got action of type %T: %v", TargetFilesChangedAction{}, a, a)
			}
			// 1. unpack to one file per action, for deterministic inspection
			// 2. make paths relative to cwd
			for _, p := range tfca.Files {
				actions = append(actions, NewTargetFilesChangedAction(tfca.TargetID, p))
				if p == lastFile {
					return actions, nil
				}
			}
		}

	}
	return nil, fmt.Errorf("timed out waiting for actions. received so far: %v", actions)
}

func (f *wmFixture) Stop(t *testing.T) []TargetFilesChangedAction {
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
	f.store.UnlockMutableState()
	f.wm.OnChange(f.ctx, f.store)
}

func (f *wmFixture) SetTiltIgnoreContents(s string) {
	state := f.store.LockMutableStateForTesting()
	state.TiltIgnoreContents = s
	f.store.UnlockMutableState()
	f.wm.OnChange(f.ctx, f.store)
}

func targetFilesChangedActionsToPaths(actions []TargetFilesChangedAction) []string {
	var paths []string
	for _, a := range actions {
		paths = append(paths, a.Files...)
	}
	return paths
}
