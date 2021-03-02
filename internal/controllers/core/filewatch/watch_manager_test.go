package filewatch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/internal/apiclient"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/clientset/tiltapi/typed/core/v1alpha1/fake"
)

func createSpec(rootPath string, matchPatterns []string, ignorePatterns []string) filewatches.FileWatchSpec {
	return filewatches.FileWatchSpec{
		RootPath:       rootPath,
		Paths:          matchPatterns,
		IgnorePatterns: ignorePatterns,
	}
}

func TestApiServerWatchManager_StartDuplicate(t *testing.T) {
	const name = "test-dupe"
	f := newWatchManagerFixture(t)

	spec := createSpec("/test", []string{"path1", "path2/path3"}, []string{"ignore1"})

	addedOrUpdated, err := f.wm.StartWatch(f.ctx, name, spec)
	require.NoError(t, err)
	require.True(t, addedOrUpdated)

	// calls to start with identical specs should return false and not replace the watch
	spec = *spec.DeepCopy()
	addedOrUpdated, err = f.wm.StartWatch(f.ctx, name, spec)
	require.NoError(t, err)
	require.False(t, addedOrUpdated)
}

func TestApiServerWatchManager_StartChanges(t *testing.T) {
	const name = "test-changes"
	f := newWatchManagerFixture(t)

	spec := createSpec("/", []string{"match"}, []string{"ignore"})

	addedOrUpdated, err := f.wm.StartWatch(f.ctx, name, spec)
	require.NoError(t, err)
	require.True(t, addedOrUpdated)

	// calls to start with identical specs should return false and not replace the watch
	spec = *spec.DeepCopy()
	spec.IgnorePatterns = append(spec.IgnorePatterns, "match2")
	addedOrUpdated, err = f.wm.StartWatch(f.ctx, name, spec)
	require.NoError(t, err)
	require.True(t, addedOrUpdated)
}

func TestApiServerWatchManager_FileEvent(t *testing.T) {
	const name = "test-file-event"
	f := newWatchManagerFixture(t)

	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	t.Cleanup(cancel)

	spec := createSpec("/a", []string{"b", "./c"}, []string{"c/d"})

	addedOrUpdated, err := f.wm.StartWatch(ctx, name, spec)
	require.NoError(t, err)
	require.True(t, addedOrUpdated)

	f.timerMaker.RestTimerLock.Lock()
	f.triggerFileEvent("/a/c/d")
	f.triggerFileEvent("/a/c/e")
	f.triggerFileEvent("/a/b/test")
	f.timerMaker.RestTimerLock.Unlock()

	f.clientFixture.AssertUpdated("status", "filewatches", func(obj runtime.Object) bool {
		updatedFwo := obj.(*filewatches.FileWatch)
		return !updatedFwo.Status.LastEventTime.IsZero() &&
			cmp.Equal(updatedFwo.Status.SeenFiles, []string{"/a/c/e", "/a/b/test"})
	})
}

func TestApiServerWatchManager_FilesystemError(t *testing.T) {
	const name = "test-filesystem-error"
	f := newWatchManagerFixture(t)

	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	t.Cleanup(cancel)

	addedOrUpdated, err := f.wm.StartWatch(ctx, name, filewatches.FileWatchSpec{})
	require.NoError(t, err)
	require.True(t, addedOrUpdated)

	f.triggerFilesystemError(errors.New("fake filesystem error"))
	f.clientFixture.AssertUpdated("status", "filewatches", func(obj runtime.Object) bool {
		updatedFwo := obj.(*filewatches.FileWatch)
		return !updatedFwo.Status.LastEventTime.IsZero() &&
			updatedFwo.Status.Error == "fake filesystem error"
	})
}

type wmFixture struct {
	t testing.TB

	ctx context.Context

	client        *fake.FakeFileWatches
	clientFixture *apiclient.FakeClientFixture
	watcherMaker  *watch.FakeMultiWatcher
	timerMaker    watch.FakeTimerMaker

	wm *filewatch.ApiServerWatchManager
}

func newWatchManagerFixture(t testing.TB) *wmFixture {
	t.Helper()

	fcf := apiclient.NewFakeClientFixture(t)
	cli := &fake.FakeFileWatches{Fake: fcf.CoreV1alpha1()}

	watcherMaker := watch.NewFakeMultiWatcher()
	timerMaker := watch.MakeFakeTimerMaker(t)

	wm := filewatch.NewApiServerWatchManager(cli, watcherMaker.NewSub, timerMaker.Maker())

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	f := &wmFixture{
		t:             t,
		client:        cli,
		clientFixture: fcf,
		ctx:           ctx,
		watcherMaker:  watcherMaker,
		timerMaker:    timerMaker,
		wm:            wm,
	}

	return f
}

func (f *wmFixture) triggerFileEvent(path string) {
	f.t.Helper()
	select {
	case f.watcherMaker.Events <- watch.NewFileEvent(path):
	default:
		f.t.Fatal("Emitting a FileEvent would block")
	}
}

func (f *wmFixture) triggerFilesystemError(err error) {
	f.t.Helper()
	select {
	case f.watcherMaker.Errors <- err:
	default:
		f.t.Fatal("Emitting a filesystem error would block")
	}
}
