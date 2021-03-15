package filewatch

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch/fsevent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/apis"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type fixture struct {
	*fake.ControllerFixture
	t                testing.TB
	ctx              context.Context
	tmpdir           *tempdir.TempDirFixture
	store            *store.TestingStore
	fakeMultiWatcher *fsevent.FakeMultiWatcher
	fakeTimerMaker   fsevent.FakeTimerMaker
}

func newFixture(t *testing.T) *fixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	tmpdir := tempdir.NewTempDirFixture(t)
	t.Cleanup(tmpdir.TearDown)
	tmpdir.Chdir()

	timerMaker := fsevent.MakeFakeTimerMaker(t)
	fakeMultiWatcher := fsevent.NewFakeMultiWatcher()

	testingStore := store.NewTestingStore()
	controller := NewController(testingStore, fakeMultiWatcher.NewSub, timerMaker.Maker())

	ctrlFixture := fake.NewFixture(t, controller)

	return &fixture{
		ControllerFixture: ctrlFixture,
		t:                 t,
		ctx:               ctx,
		tmpdir:            tmpdir,
		store:             testingStore,
		fakeMultiWatcher:  fakeMultiWatcher,
		fakeTimerMaker:    timerMaker,
	}
}

func (f *fixture) ChangeFile(t testing.TB, path string) {
	path, _ = filepath.Abs(path)

	select {
	case f.fakeMultiWatcher.Events <- watch.NewFileEvent(path):
	default:
		t.Fatal("emitting a FileEvent would block. Perhaps there are too many events or the buffer size is too small.")
	}
}

func (f *fixture) WaitForSeenFile(name string, path string) []string {
	f.t.Helper()
	var seenPaths []string
	require.Eventuallyf(f.t, func() bool {
		seenPaths = nil
		var fw filewatches.FileWatch
		if !f.Get(name, &fw) {
			return false
		}
		found := false
		for _, e := range fw.Status.FileEvents {
			for _, p := range e.SeenFiles {
				p, _ = filepath.Rel(f.tmpdir.Path(), p)
				if p == path {
					found = true
				}
				seenPaths = append(seenPaths, p)
			}
		}
		return found
	}, 2*time.Second, 20*time.Millisecond, "Did not find path %q, seen: %v", path, &seenPaths)
	return seenPaths
}

func (f *fixture) CreateSimpleFileWatch() *filewatches.FileWatch {
	name := apis.SanitizeName(f.t.Name())
	fw := &filewatches.FileWatch{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: filewatches.FileWatchSpec{
			WatchedPaths: []string{f.tmpdir.Path()},
		},
	}
	f.Create(fw)
	return fw
}

func TestController_LimitFileEventsHistory(t *testing.T) {
	f := newFixture(t)

	fw := f.CreateSimpleFileWatch()

	const eventOverflowCount = 5
	for i := 0; i < MaxFileEventHistory+eventOverflowCount; i++ {
		p := strconv.Itoa(i)
		f.ChangeFile(t, p)
		// need to wait for each file 1-by-1 to prevent batching
		f.WaitForSeenFile(fw.Name, p)
	}

	f.MustGet(fw.Name, fw)
	require.Equal(t, MaxFileEventHistory, len(fw.Status.FileEvents), "Wrong number of file events")
	for i := 0; i < len(fw.Status.FileEvents); i++ {
		p := f.tmpdir.JoinPath(strconv.Itoa(i + eventOverflowCount))
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
