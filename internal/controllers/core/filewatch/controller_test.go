package filewatch

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/ctrltest"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func (f *fixture) newFileWatch(name string, watchPaths []string, ignorePatterns []string) *filewatches.FileWatch {
	return &filewatches.FileWatch{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: filewatches.FileWatchSpec{
			Watches: []filewatches.WatchDef{
				{
					RootPath:       f.dir.Path(),
					Paths:          watchPaths,
					IgnorePatterns: ignorePatterns,
				},
			},
		},
	}
}

func TestCreateWatch(t *testing.T) {
	f := newFixture(t)

	fw := f.newFileWatch("test", []string{"a/b", "a/c"}, []string{"a/c/d"})

	f.Create(fw)

	f.requireWatchedPathsEqual("a/b", "a/c")
	f.requireIgnorePatternsMatch("a/c/d/e")
}

func TestUpdateWatch(t *testing.T) {
	f := newFixture(t)

	fw := f.newFileWatch("test", []string{"a/b", "a/c"}, []string{"a/c/d"})

	f.Create(fw)

	forUpdate := &filewatches.FileWatch{}
	fw.DeepCopyInto(forUpdate)
	forUpdate.Spec.Watches[0].Paths = []string{"a/c", "z/y"}

	f.Update(forUpdate)

	f.requireWatchedPathsEqual("a/c", "z/y")
	f.requireIgnorePatternsMatch("a/c/d/e")
}

func TestDeleteWatch(t *testing.T) {
	f := newFixture(t)

	fw := f.newFileWatch("test", []string{"a/b", "a/c"}, []string{"a/c/d"})

	f.Create(fw)
	f.requireWatchedPathsEqual("a/b", "a/c")

	f.Delete(fw)
	require.Empty(t, f.fakeWatcher.AllWatchPaths())
}

func TestFileEvents(t *testing.T) {
	f := newFixture(t)

	fw := f.newFileWatch("test", []string{"a/b", "a/c"}, []string{"a/c/d"})

	f.Create(fw)

	f.triggerFileEvent("a/b/match", "a/c/d/ignore")

	updatedStatus := f.waitForFileEventCount(1)

	require.NotZero(t, updatedStatus.LastEventTime)
	event := updatedStatus.FileEvents[0]
	assert.NotZero(t, event.Time, ".status.fileEvents[0].time not populated")
	assert.Equal(t,
		[]string{f.dir.JoinPath("a/b/match")},
		event.SeenFiles,
		".status.fileEvents[0].seenFiles not as expected")
}

type fixture struct {
	*ctrltest.Fixture

	t testing.TB

	controller *Controller

	dir *tempdir.TempDirFixture

	fakeWatcher    *watch.FakeMultiWatcher
	fakeTimerMaker watch.FakeTimerMaker
}

func newFixture(t testing.TB) *fixture {
	wm := watch.NewFakeMultiWatcher()
	tm := watch.MakeFakeTimerMaker(t)

	c := NewController(wm.NewSub, tm.Maker())
	baseFixture := ctrltest.NewFixture(t, c)

	tempFixture := tempdir.NewTempDirFixture(t)
	t.Cleanup(tempFixture.TearDown)

	return &fixture{
		t:              t,
		Fixture:        baseFixture,
		controller:     c,
		dir:            tempFixture,
		fakeWatcher:    wm,
		fakeTimerMaker: tm,
	}
}

func (f *fixture) requireWatchedPathsEqual(pathPartials ...string) {
	f.t.Helper()
	actualWatchedAbsPaths := f.fakeWatcher.AllWatchPaths()
	var expectedAbsPaths []string
	for _, p := range pathPartials {
		expectedAbsPaths = append(expectedAbsPaths, f.dir.JoinPath(p))
	}

	// there are no order guarantees, so alpha sort both collections
	sort.Strings(expectedAbsPaths)
	sort.Strings(actualWatchedAbsPaths)

	require.Equal(f.t, expectedAbsPaths, actualWatchedAbsPaths, "Watched paths did not match")
}

func (f *fixture) requireIgnorePatternsMatch(pathPartials ...string) {
	f.t.Helper()
	for _, p := range pathPartials {
		absPath := f.dir.JoinPath(p)
		m, err := f.fakeWatcher.IgnorePatternMatches(absPath)
		require.NoErrorf(f.t, err, "Error executing matcher for %q", absPath)
		if !m {
			f.t.Fatalf("Path was not matched for ignore: %q", absPath)
		}
	}
}

func (f *fixture) triggerFileEvent(pathPartials ...string) {
	f.fakeTimerMaker.MaxTimerLock.Lock()
	defer f.fakeTimerMaker.MaxTimerLock.Unlock()
	f.fakeTimerMaker.RestTimerLock.Lock()
	defer f.fakeTimerMaker.RestTimerLock.Unlock()
	for _, p := range pathPartials {
		f.fakeWatcher.Events <- watch.NewFileEvent(f.dir.JoinPath(p))
	}
}

func (f *fixture) waitForFileEventCount(count int) filewatches.FileWatchStatus {
	var fw filewatches.FileWatch
	require.Eventually(f.t, func() bool {
		if !f.Get("test", &fw) {
			return false
		}
		return len(fw.Status.FileEvents) >= count
	}, time.Second, 20*time.Millisecond, "No file events received")
	return fw.Status
}
