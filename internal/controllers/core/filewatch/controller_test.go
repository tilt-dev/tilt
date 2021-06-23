package filewatch

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch/fsevent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/apis"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type fixture struct {
	*fake.ControllerFixture
	t                testing.TB
	tmpdir           *tempdir.TempDirFixture
	controller       *Controller
	store            *store.TestingStore
	fakeMultiWatcher *fsevent.FakeMultiWatcher
	fakeTimerMaker   fsevent.FakeTimerMaker
}

func newFixture(t *testing.T) *fixture {
	tmpdir := tempdir.NewTempDirFixture(t)
	t.Cleanup(tmpdir.TearDown)
	tmpdir.Chdir()

	timerMaker := fsevent.MakeFakeTimerMaker(t)
	fakeMultiWatcher := fsevent.NewFakeMultiWatcher()

	testingStore := store.NewTestingStore()

	cfb := fake.NewControllerFixtureBuilder(t)
	controller := NewController(cfb.Client, testingStore, fakeMultiWatcher.NewSub, timerMaker.Maker())

	return &fixture{
		ControllerFixture: cfb.Build(controller),
		t:                 t,
		tmpdir:            tmpdir,
		controller:        controller,
		store:             testingStore,
		fakeMultiWatcher:  fakeMultiWatcher,
		fakeTimerMaker:    timerMaker,
	}
}

func (f *fixture) ChangeAndWaitForSeenFile(key types.NamespacedName, pathElems ...string) {
	f.t.Helper()
	f.ChangeFile(pathElems...)
	f.WaitForSeenFile(key, pathElems...)
}

func (f *fixture) ChangeFile(elem ...string) {
	f.t.Helper()
	path, err := filepath.Abs(f.tmpdir.JoinPath(elem...))
	require.NoErrorf(f.t, err, "Could not get abs path for %q", path)
	select {
	case f.fakeMultiWatcher.Events <- watch.NewFileEvent(path):
	default:
		f.t.Fatal("emitting a FileEvent would block. Perhaps there are too many events or the buffer size is too small.")
	}
}

func (f *fixture) WaitForSeenFile(key types.NamespacedName, pathElems ...string) {
	f.t.Helper()
	relPath := filepath.Join(pathElems...)
	var seenPaths []string
	require.Eventuallyf(f.t, func() bool {
		seenPaths = nil
		var fw filewatches.FileWatch
		if !f.Get(key, &fw) {
			return false
		}
		found := false
		for _, e := range fw.Status.FileEvents {
			for _, p := range e.SeenFiles {
				// relativize all the paths before comparison/storage
				// (this makes the test output way more comprehensible on failure by hiding all the tmpdir cruft)
				p, _ = filepath.Rel(f.tmpdir.Path(), p)
				if p == relPath {
					found = true
				}
				seenPaths = append(seenPaths, p)
			}
		}
		return found
	}, 2*time.Second, 20*time.Millisecond, "Did not find path %q, seen: %v", relPath, &seenPaths)
}

func (f *fixture) CreateSimpleFileWatch() (types.NamespacedName, *filewatches.FileWatch) {
	f.t.Helper()
	fw := &filewatches.FileWatch{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: apis.SanitizeName(f.t.Name()),
			Name:      "test-file-watch",
		},
		Spec: filewatches.FileWatchSpec{
			WatchedPaths: []string{f.tmpdir.JoinPath("a"), f.tmpdir.JoinPath("b", "c")},
		},
	}
	f.Create(fw)
	return f.KeyForObject(fw), fw
}

func TestController_LimitFileEventsHistory(t *testing.T) {
	f := newFixture(t)

	key, fw := f.CreateSimpleFileWatch()

	const eventOverflowCount = 5
	for i := 0; i < MaxFileEventHistory+eventOverflowCount; i++ {
		// need to wait for each file 1-by-1 to prevent batching
		f.ChangeAndWaitForSeenFile(key, "a", strconv.Itoa(i))
	}

	f.MustGet(key, fw)
	require.Equal(t, MaxFileEventHistory, len(fw.Status.FileEvents), "Wrong number of file events")
	for i := 0; i < len(fw.Status.FileEvents); i++ {
		p := f.tmpdir.JoinPath("a", strconv.Itoa(i+eventOverflowCount))
		assert.Contains(t, fw.Status.FileEvents[i].SeenFiles, p)
	}
}

func TestController_ShortRead(t *testing.T) {
	f := newFixture(t)
	f.CreateSimpleFileWatch()

	f.fakeMultiWatcher.Errors <- fmt.Errorf("short read on readEvents()")

	errorAction := f.store.WaitForAction(t, reflect.TypeOf(store.ErrorAction{}))
	storeErr := errorAction.(store.ErrorAction).Error

	if assert.Contains(t, storeErr.Error(), "short read") && runtime.GOOS == "windows" {
		assert.Contains(t, storeErr.Error(), "https://github.com/tilt-dev/tilt/issues/3556")
	}
}

func TestController_IgnoreEphemeralFiles(t *testing.T) {
	f := newFixture(t)
	key, orig := f.CreateSimpleFileWatch()
	// spec should have no ignores - these are purely implicit ignores
	require.Empty(t, orig.Spec.Ignores)

	// sandwich in some ignored files with seen files on the outside as synchronization
	f.ChangeAndWaitForSeenFile(key, "a", "start")
	// see internal/ignore/ephemeral.go for where these come from - they're NOT part of a FileWatch spec
	// but are always included at the filesystem watcher level by Tilt
	f.ChangeFile("a", ".idea", "workspace.xml")
	f.ChangeFile("b", "c", ".vim.swp")
	f.ChangeAndWaitForSeenFile(key, "b", "c", "stop")

	var fw filewatches.FileWatch
	f.MustGet(key, &fw)
	require.Equal(t, 2, len(fw.Status.FileEvents), "Wrong file event count")
	assert.Equal(t, []string{f.tmpdir.JoinPath("a", "start")}, fw.Status.FileEvents[0].SeenFiles)
	assert.Equal(t, []string{f.tmpdir.JoinPath("b", "c", "stop")}, fw.Status.FileEvents[1].SeenFiles)
}

// TestController_Watcher_Cancel peeks into internal/unexported portions of the controller to inspect the actual
// filesystem monitor so it can ensure reconciler is not leaking resources; other tests should prefer observing
// desired state!
func TestController_Watcher_Cancel(t *testing.T) {
	f := newFixture(t)
	key, _ := f.CreateSimpleFileWatch()

	assert.Equalf(t, 1, len(f.controller.targetWatches), "There should be exactly one file watcher")
	watcher := f.controller.targetWatches[key]
	require.NotNilf(t, watcher, "Watcher does not exist for %q", key.String())

	// cancel the root context, which should propagate to the watcher
	f.Cancel()

	require.Eventuallyf(t, func() bool {
		watcher.mu.Lock()
		defer watcher.mu.Unlock()
		return watcher.done
	}, time.Second, 10*time.Millisecond, "Watcher was never cleaned up")
}

func TestController_Reconcile_Create(t *testing.T) {
	f := newFixture(t)
	key, fw := f.CreateSimpleFileWatch()

	f.MustGet(key, fw)
	assert.NotZero(t, fw.Status.MonitorStartTime, "Filesystem monitor was not started")
}

// TestController_Reconcile_Delete peeks into internal/unexported portions of the controller to inspect the actual
// filesystem monitor so it can ensure reconciler is not leaking resources; other tests should prefer observing
// desired state!
func TestController_Reconcile_Delete(t *testing.T) {
	f := newFixture(t)
	key, fw := f.CreateSimpleFileWatch()

	assert.Equalf(t, 1, len(f.controller.targetWatches), "There should be exactly one file watcher")
	watcher := f.controller.targetWatches[key]
	require.NotNilf(t, watcher, "Watcher does not exist for %q", key.String())

	deleted, _ := f.Delete(fw)
	require.True(t, deleted, "FileWatch was not deleted")

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	require.True(t, watcher.done, "Watcher was not stopped")
	require.Empty(t, f.controller.targetWatches, "There should not be any remaining file watchers")
}

func TestController_Reconcile_Watches(t *testing.T) {
	f := newFixture(t)
	key, fw := f.CreateSimpleFileWatch()

	f.ChangeAndWaitForSeenFile(key, "a", "1")

	f.MustGet(key, fw)
	originalStart := fw.Status.MonitorStartTime.Time
	assert.NotZero(t, originalStart, "Filesystem monitor was not started")

	fw.Spec.Ignores = []filewatches.IgnoreDef{
		{
			BasePath: f.tmpdir.Path(),
			Patterns: []string{"**/ignore_me"},
		},
		{
			// no patterns means ignore the path recursively
			BasePath: f.tmpdir.JoinPath("d", "ignore_dir"),
		},
	}
	fw.Spec.WatchedPaths = []string{f.tmpdir.JoinPath("d")}
	f.Update(fw)

	// sandwich in some ignored files with seen files on the outside as synchronization
	f.ChangeAndWaitForSeenFile(key, "d", "1")
	f.ChangeFile("a", "2")
	f.ChangeFile("d", "ignore_me")
	f.ChangeFile("d", "ignore_dir", "file")
	f.ChangeAndWaitForSeenFile(key, "d", "2")

	var updated filewatches.FileWatch
	f.MustGet(key, &updated)
	updatedStart := updated.Status.MonitorStartTime.Time
	assert.Truef(t, updatedStart.After(originalStart),
		"Monitor start time should be more recent after update, (original: %s, updated: %s)",
		originalStart, updatedStart)
	if assert.Equal(t, 2, len(updated.Status.FileEvents)) {
		// ensure ONLY the expected files were seen
		assert.NotZero(t, updated.Status.FileEvents[0].Time.Time)
		mostRecentEventTime := updated.Status.FileEvents[1].Time.Time
		assert.NotZero(t, mostRecentEventTime)
		assert.Equal(t, []string{f.tmpdir.JoinPath("d", "1")}, updated.Status.FileEvents[0].SeenFiles)
		assert.Equal(t, []string{f.tmpdir.JoinPath("d", "2")}, updated.Status.FileEvents[1].SeenFiles)
		assert.Equal(t, mostRecentEventTime, updated.Status.LastEventTime.Time)
	}
}
