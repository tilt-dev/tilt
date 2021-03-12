package fswatch

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestWatchManager_basic(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{WatchedPaths: []string{"."}})

	f.ChangeFile(t, "foo.txt")

	seenPaths := f.Stop()
	assert.Contains(t, seenPaths, "foo.txt")
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

	seenPaths := f.seenPaths()
	assert.NotContains(t, seenPaths, "foo.txt")
}

func TestWatchManager_IgnoredLocalDirectories(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithIgnoredLocalDirectories([]string{"bar"}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: "bar"},
		},
	})

	f.ChangeFile(t, filepath.Join("bar", "baz"))

	seenPaths := f.Stop()
	assert.NotContains(t, seenPaths, filepath.Join("bar", "baz"))
}

func TestWatchManager_Dockerignore(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithDockerignores([]model.Dockerignore{{LocalPath: ".", Patterns: []string{"bar"}}}).
		WithBuildPath(".")
	f.SetManifestTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: ".", Patterns: []string{"bar"}},
		},
	})

	f.ChangeFile(t, filepath.Join("bar", "baz"))

	seenPaths := f.Stop()
	assert.NotContains(t, seenPaths, filepath.Join("bar", "baz"))
}

func TestWatchManager_IgnoreOutputsImageRefs(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.ImageTarget{}.WithBuildDetails(model.CustomBuild{
		Deps:              []string{f.Path()},
		OutputsImageRefTo: f.JoinPath("ref.txt"),
	})

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTarget(target).
		Build()

	st := f.store.LockMutableStateForTesting()
	st.UpsertManifestTarget(store.NewManifestTarget(m))
	f.store.UnlockMutableState()

	// simulate an action to ensure subscribers see changes
	f.store.Dispatch(configs.ConfigsReloadedAction{})

	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{f.Path()},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"ref.txt"}},
		},
	})

	f.ChangeFile(t, "included.txt")
	f.ChangeFile(t, "ref.txt")

	seenPaths := f.Stop()
	assert.Contains(t, seenPaths, "included.txt")
	assert.NotContains(t, seenPaths, "ref.txt")
}

func TestWatchManager_WatchesReappliedOnDockerComposeSyncChange(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target.WithIgnoredLocalDirectories([]string{"bar"}))
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: "bar"},
		},
	})

	f.SetManifestTarget(target)
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{WatchedPaths: []string{"."}})

	f.ChangeFile(t, "bar")

	seenPaths := f.Stop()
	// not asserting exact contents because we can end up with duplicates since the old watch loop isn't stopped
	// until after the new watch loop is started
	assert.Contains(t, seenPaths, "bar")
}

func TestWatchManager_WatchesReappliedOnDockerIgnoreChange(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target.WithDockerignores([]model.Dockerignore{{LocalPath: ".", Patterns: []string{"bar"}}}))
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: ".", Patterns: []string{"bar"}},
		},
	})

	f.SetManifestTarget(target)
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{WatchedPaths: []string{"."}})

	f.ChangeFile(t, "bar")

	seenPaths := f.Stop()
	// not asserting exact contents because we can end up with duplicates since the old watch loop isn't stopped
	// until after the new watch loop is started
	assert.Contains(t, seenPaths, "bar")
}

func TestWatchManager_ConfigFiles(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	f.SetTiltIgnoreContents("**/foo")
	st := f.store.LockMutableStateForTesting()
	// N.B. because there's no target for this test watching `.` need to set
	//	an explicit watch on `stop` for the test fixture
	st.ConfigFiles = append(st.ConfigFiles, "path_to_watch", "stop")
	f.store.UnlockMutableState()
	f.store.Dispatch(configs.ConfigsReloadedAction{})

	f.RequireFileWatchSpecEqual(ConfigsTargetID, filewatches.FileWatchSpec{
		WatchedPaths: []string{"path_to_watch", "stop"},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})

	f.ChangeFile(t, filepath.Join("path_to_watch", "foo"))
	f.ChangeFile(t, filepath.Join("path_to_watch", "bar"))

	seenPaths := f.Stop()
	assert.NotContains(t, seenPaths, filepath.Join("path_to_watch", "foo"))
	assert.Contains(t, seenPaths, filepath.Join("path_to_watch", "bar"))
}

func TestWatchManager_IgnoreTiltIgnore(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)
	f.SetTiltIgnoreContents("**/foo")
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})

	f.ChangeFile(t, filepath.Join("bar", "foo"))

	seenPaths := f.Stop()
	assert.NotContains(t, seenPaths, filepath.Join("bar", "foo"))
}

func TestWatchManager_IgnoreWatchSettings(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)

	st := f.store.LockMutableStateForTesting()
	st.WatchSettings.Ignores = append(st.WatchSettings.Ignores, model.Dockerignore{
		LocalPath: f.Path(),
		Patterns:  []string{"**/foo"},
	})
	f.store.UnlockMutableState()
	// simulate an action to ensure subscribers see changes
	f.store.Dispatch(configs.ConfigsReloadedAction{})

	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})

	f.ChangeFile(t, filepath.Join("bar", "foo"))

	seenPaths := f.Stop()
	assert.NotContains(t, seenPaths, filepath.Join("bar", "foo"))
}

func TestWatchManager_PickUpTiltIgnoreChanges(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)
	f.SetTiltIgnoreContents("**/foo")
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})
	f.ChangeFile(t, filepath.Join("bar", "foo"))

	f.SetTiltIgnoreContents("**foo\n!bar/baz/foo")
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**foo", "!bar/baz/foo"}},
		},
	})
	f.ChangeFile(t, filepath.Join("bar", "baz", "foo"))

	seenPaths := f.Stop()
	assert.NotContains(t, seenPaths, filepath.Join("bar", "foo"))
	assert.Contains(t, seenPaths, filepath.Join("bar", "baz", "foo"))
}

func TestWatchManagerShortRead(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestTarget(target)
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{WatchedPaths: []string{"."}})

	f.fakeMultiWatcher.Errors <- fmt.Errorf("short read on readEvents()")

	assert.Eventually(t, func() bool {
		storeErr := f.storeError()
		if storeErr == nil {
			return false
		}
		isShortRead := strings.Contains(storeErr.Error(), "short read")
		if isShortRead && runtime.GOOS == "windows" {
			isShortRead = strings.Contains(storeErr.Error(), "https://github.com/tilt-dev/tilt/issues/3556")
		}
		return isShortRead
	}, time.Second, 10*time.Millisecond, "Short read error was not found")
}

type wmFixture struct {
	t                testing.TB
	ctx              context.Context
	store            *store.Store
	storeErr         atomic.Value
	wm               *WatchManager
	fakeMultiWatcher *FakeMultiWatcher
	fakeTimerMaker   FakeTimerMaker
	*tempdir.TempDirFixture
}

func newWMFixture(t *testing.T) *wmFixture {
	timerMaker := MakeFakeTimerMaker(t)
	fakeMultiWatcher := NewFakeMultiWatcher()
	wm := NewWatchManager(fakeMultiWatcher.NewSub, timerMaker.Maker())

	manifestSub := NewManifestSubscriber()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	tmpdir := tempdir.NewTempDirFixture(t)
	tmpdir.Chdir()

	f := &wmFixture{
		t:                t,
		ctx:              ctx,
		wm:               wm,
		fakeMultiWatcher: fakeMultiWatcher,
		fakeTimerMaker:   timerMaker,
		TempDirFixture:   tmpdir,
	}

	f.store = store.NewStore(f.reducer, false)
	require.NoError(t, f.store.AddSubscriber(f.ctx, wm))
	require.NoError(t, f.store.AddSubscriber(f.ctx, manifestSub))

	go func() {
		if err := f.store.Loop(ctx); err != nil && err != context.Canceled {
			f.storeErr.Store(err)
		}
	}()

	t.Cleanup(func() {
		tmpdir.TearDown()
		cancel()
	})

	return f
}

func (f *wmFixture) reducer(ctx context.Context, st *store.EngineState, action store.Action) {
	switch a := action.(type) {
	case store.ErrorAction:
		f.storeErr.Store(a.Error)
	case store.LogAction:
		f.t.Log(a.String())
	case FileWatchCreateAction:
		HandleFileWatchCreateEvent(ctx, st, a)
	case FileWatchUpdateAction:
		HandleFileWatchUpdateEvent(ctx, st, a)
	case FileWatchUpdateStatusAction:
		HandleFileWatchUpdateStatusEvent(ctx, st, a)
	case FileWatchDeleteAction:
		HandleFileWatchDeleteEvent(ctx, st, a)
	}
}

func (f *wmFixture) ChangeFile(t testing.TB, path string) {
	path, _ = filepath.Abs(path)

	select {
	case f.fakeMultiWatcher.Events <- watch.NewFileEvent(path):
	default:
		t.Fatal("emitting a FileEvent would block. Perhaps there are too many events or the buffer size is too small.")
	}
}

func (f *wmFixture) seenPaths() []string {
	st := f.store.RLockState()
	defer f.store.RUnlockState()

	var seen []string

	for _, fw := range st.FileWatches {
		for _, e := range fw.Status.FileEvents {
			for _, p := range e.SeenFiles {
				p, _ = filepath.Rel(f.TempDirFixture.Path(), p)
				seen = append(seen, p)
			}
		}
	}
	return seen
}

type fileWatchDiffer struct {
	expected filewatches.FileWatchSpec
	actual   *filewatches.FileWatchSpec
}

func (f fileWatchDiffer) String() string {
	return cmp.Diff(f.actual, &f.expected)
}

func (f *wmFixture) RequireFileWatchSpecEqual(targetID model.TargetID, spec filewatches.FileWatchSpec) {
	f.t.Helper()
	fwd := &fileWatchDiffer{expected: spec}
	require.Eventuallyf(f.t, func() bool {
		fwd.actual = nil
		st := f.store.RLockState()
		defer f.store.RUnlockState()
		fw, ok := st.FileWatches[types.NamespacedName{Name: targetID.String()}]
		if !ok {
			return false
		}
		fwd.actual = fw.Spec.DeepCopy()
		return equality.Semantic.DeepEqual(fw.Spec, spec)
	}, time.Second, 10*time.Millisecond, "FileWatch spec was not equal: %v", fwd)
}

func (f *wmFixture) WaitForSeenFile(path string) []string {
	f.t.Helper()
	var seenPaths []string
	require.Eventuallyf(f.t, func() bool {
		seenPaths = f.seenPaths()
		for _, p := range seenPaths {
			if p == path {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "Did not find path %q, seen: %v", path, &seenPaths)
	return seenPaths
}

func (f *wmFixture) storeError() error {
	err := f.storeErr.Load()
	if err != nil {
		return err.(error)
	}
	return nil
}

func (f *wmFixture) Stop() []string {
	f.t.Helper()
	f.ChangeFile(f.t, "stop")
	seenPaths := f.WaitForSeenFile("stop")
	require.NoError(f.t, f.storeError(), "Store encountered error action")
	return seenPaths
}

func (f *wmFixture) SetManifestTarget(target model.DockerComposeTarget) {
	m := model.Manifest{Name: "foo"}.WithDeployTarget(target)
	mt := store.NewManifestTarget(m)
	state := f.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(mt)
	f.store.UnlockMutableState()
	// simulate an action to ensure subscribers see changes
	f.store.Dispatch(configs.ConfigsReloadedAction{})
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
	// simulate an action to ensure subscribers see changes
	f.store.Dispatch(configs.ConfigsReloadedAction{})
}
