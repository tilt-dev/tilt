package fswatch

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
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
}

func TestWatchManager_localRepo(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".").
		WithRepos([]model.LocalGitRepo{model.LocalGitRepo{LocalPath: "."}})
	f.SetManifestTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: filepath.Join(".", ".git")},
		},
	})
}

func TestWatchManager_disabledOnCIMode(t *testing.T) {
	testingStore := store.NewTestingStore()

	state := testingStore.LockMutableStateForTesting()

	state.EngineMode = store.EngineModeCI

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	m := model.Manifest{Name: "foo"}.WithDeployTarget(target)
	mt := store.NewManifestTarget(m)
	state.UpsertManifestTarget(mt)

	testingStore.UnlockMutableState()

	cli := fake.NewTiltClient()
	ms := NewManifestSubscriber(cli)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	_ = ms.OnChange(ctx, testingStore, store.LegacyChangeSummary())

	assert.Empty(t, testingStore.Actions())
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
}

func TestWatchManager_IgnoreOutputsImageRefs(t *testing.T) {
	f := newWMFixture(t)
	defer f.TearDown()

	target := model.MustNewImageTarget(container.MustParseSelector("img")).
		WithBuildDetails(model.CustomBuild{
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

	f.SetTiltIgnoreContents("**foo\n!bar/baz/foo")
	f.RequireFileWatchSpecEqual(target.ID(), filewatches.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []filewatches.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**foo", "!bar/baz/foo"}},
		},
	})
}

type wmFixture struct {
	t        testing.TB
	ctx      context.Context
	store    *store.Store
	storeErr atomic.Value
	cli      ctrlclient.Client
	*tempdir.TempDirFixture
}

func newWMFixture(t *testing.T) *wmFixture {
	cli := fake.NewTiltClient()
	manifestSub := NewManifestSubscriber(cli)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	tmpdir := tempdir.NewTempDirFixture(t)
	tmpdir.Chdir()

	f := &wmFixture{
		t:              t,
		ctx:            ctx,
		cli:            cli,
		TempDirFixture: tmpdir,
	}

	// TODO(milas): this can be changed to the testing store to eliminate async
	f.store = store.NewStore(f.reducer, false)
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

type fileWatchDiffer struct {
	expected filewatches.FileWatchSpec
	actual   *filewatches.FileWatchSpec
}

func (f fileWatchDiffer) String() string {
	return cmp.Diff(&f.expected, f.actual)
}

func (f *wmFixture) RequireEventually(cond func() bool, msg string, args ...interface{}) {
	require.Eventuallyf(f.t, func() bool {
		storeErr := f.storeError()
		if storeErr != nil {
			assert.FailNow(f.t, fmt.Sprintf("store encountered fatal error: %v", storeErr), append([]interface{}{msg}, args...)...)
		}
		return cond()
	}, time.Second, 10*time.Millisecond, msg, args...)
}

func (f *wmFixture) RequireFileWatchSpecEqual(targetID model.TargetID, spec filewatches.FileWatchSpec) {
	f.t.Helper()
	fwd := &fileWatchDiffer{expected: spec}
	f.RequireEventually(func() bool {
		fwd.actual = nil
		st := f.store.RLockState()
		defer f.store.RUnlockState()
		fw, ok := st.FileWatches[keyForTarget(targetID)]
		if !ok {
			return false
		}
		fwd.actual = fw.Spec.DeepCopy()
		return equality.Semantic.DeepEqual(fw.Spec, spec)
	}, "FileWatch spec was not equal: %v", fwd)
}

func (f *wmFixture) storeError() error {
	err := f.storeErr.Load()
	if err != nil {
		return err.(error)
	}
	return nil
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

func keyForTarget(targetID model.TargetID) types.NamespacedName {
	return types.NamespacedName{Name: apis.SanitizeName(targetID.String())}
}
