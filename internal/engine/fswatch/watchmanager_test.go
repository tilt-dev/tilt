package fswatch

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
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

	store.AssertNoActionOfType(t, reflect.TypeOf(FileWatchUpdateAction{}), f.store.Actions)
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
		WithDockerignores([]model.Dockerignore{{LocalPath: ".", Patterns: []string{"bar"}}}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.ChangeFile(t, filepath.Join("bar", "baz"))

	actions := f.Stop(t)

	f.AssertActionsNotContain(actions, filepath.Join("bar", "baz"))
}

func TestWatchManager_IgnoreOutputsImageRefs(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "sancho").
			WithK8sYAML(testyaml.SanchoYAML).
			WithImageTarget(
				model.ImageTarget{}.WithBuildDetails(model.CustomBuild{
					Deps:              []string{f.Path()},
					OutputsImageRefTo: f.JoinPath("ref.txt"),
				})).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))
	})
	f.wm.OnChange(f.ctx, f.store)

	f.ChangeFile(t, "included.txt")
	f.ChangeFile(t, "ref.txt")
	actions := f.Stop(t)
	f.AssertActionsNotContain(actions, "ref.txt")
	f.AssertActionsContain(actions, "included.txt")
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
	f.SetManifestTarget(target.WithDockerignores([]model.Dockerignore{{LocalPath: ".", Patterns: []string{"bar"}}}))
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

func TestWatchManager_IgnoreWatchSettings(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.store.WithState(func(es *store.EngineState) {
		es.WatchSettings.Ignores = append(es.WatchSettings.Ignores, model.Dockerignore{
			LocalPath: f.Path(),
			Patterns:  []string{"**/foo"},
		})
	})
	f.wm.OnChange(f.ctx, f.store)

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

func TestWatchManagerShortRead(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.fakeMultiWatcher.Errors <- fmt.Errorf("short read on readEvents()")

	action := f.store.WaitForAction(t, reflect.TypeOf(store.ErrorAction{}))
	msg := action.(store.ErrorAction).Error.Error()
	assert.Contains(t, msg, "short read")
	if runtime.GOOS == "windows" {
		assert.Contains(t, msg, "https://github.com/tilt-dev/tilt/issues/3556")
	}
	f.store.ClearActions()
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

func (f *wmFixture) AssertActionsContain(actions []store.Action, path string) {
	path, _ = filepath.Abs(path)
	observedPaths := extractSeenPathsFromFileWatchActions(actions)
	assert.Contains(f.T(), observedPaths, path)
}

func (f *wmFixture) AssertActionsNotContain(actions []store.Action, path string) {
	path, _ = filepath.Abs(path)
	observedPaths := extractSeenPathsFromFileWatchActions(actions)
	assert.NotContains(f.T(), observedPaths, path)
}

func (f *wmFixture) ReadActionsUntil(lastFile string) ([]store.Action, error) {
	lastFile, _ = filepath.Abs(lastFile)
	startTime := time.Now()
	timeout := 5 * time.Second
	var actions []store.Action
	for time.Since(startTime) < timeout {
		actions = f.store.Actions()
		seenPaths := extractSeenPathsFromFileWatchActions(actions)
		for _, p := range seenPaths {
			if lastFile == p {
				return actions, nil
			}
		}
	}
	return nil, fmt.Errorf("timed out waiting for actions. received so far: %v", actions)
}

func (f *wmFixture) Stop(t *testing.T) []store.Action {
	t.Helper()
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

	patterns, err := dockerignore.ReadAll(strings.NewReader(s))
	assert.NoError(f.T(), err)
	state.Tiltignore = model.Dockerignore{
		LocalPath: f.Path(),
		Patterns:  patterns,
	}
	f.store.UnlockMutableState()
	f.wm.OnChange(f.ctx, f.store)
}

func extractSeenPathsFromFileWatchActions(actions []store.Action) []string {
	var paths []string
	for _, a := range actions {
		var actionStatus *filewatches.FileWatchStatus
		switch action := a.(type) {
		case FileWatchCreateAction:
			actionStatus = &action.FileWatch.Status
		case FileWatchUpdateAction:
			actionStatus = &action.FileWatch.Status
		case FileWatchUpdateStatusAction:
			actionStatus = action.Status
		case FileWatchDeleteAction:
		}
		if actionStatus != nil {
			for _, e := range actionStatus.FileEvents {
				paths = append(paths, e.SeenFiles...)
			}
		}
	}
	return paths
}
