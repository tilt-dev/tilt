package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/fsnotify"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestServiceWatcher(t *testing.T) {
	tf := makeServiceWatcherTestFixture(t, 1)

	services := []model.Service{
		{Name: model.ServiceName("service1"), Mounts: []model.Mount{tf.mounts[0]}},
	}

	sw, err := makeServiceWatcher(tf.ctx, tf.watcherMaker, tf.timerMaker.maker(), services)

	if err != nil {
		t.Fatal(err)
	}

	tf.timerMaker.maxTimerLock.Lock()
	f1 := tf.tempDirs[0].JoinPath("foo")
	tf.watchers[0].events <- fsnotify.Event{Name: f1}
	event := <-sw.events
	assert.Equal(t, event.service, services[0])
	assert.Equal(t, []string{f1}, event.files)
}

func TestServiceWatcherTwoServices(t *testing.T) {
	tf := makeServiceWatcherTestFixture(t, 2)

	services := []model.Service{
		{Name: model.ServiceName("service1"), Mounts: []model.Mount{tf.mounts[0]}},
		{Name: model.ServiceName("service2"), Mounts: []model.Mount{tf.mounts[1]}},
	}

	sw, err := makeServiceWatcher(tf.ctx, tf.watcherMaker, tf.timerMaker.maker(), services)

	if err != nil {
		t.Fatal(err)
	}

	tf.timerMaker.maxTimerLock.Lock()
	f1 := writeEvent(t, tf.watchers[0], tf.tempDirs[0])
	event := <-sw.events
	assert.Equal(t, services[0], event.service)
	assert.Equal(t, []string{f1}, event.files)
	tf.timerMaker.restTimerLock.Lock()
	f2 := writeEvent(t, tf.watchers[1], tf.tempDirs[1])
	f3 := writeEvent(t, tf.watchers[0], tf.tempDirs[0])
	f4 := writeEvent(t, tf.watchers[1], tf.tempDirs[1])
	event = <-sw.events
	assert.Equal(t, services[1], event.service)
	assert.Equal(t, []string{f2, f4}, event.files)
	event = <-sw.events
	assert.Equal(t, services[0], event.service)
	assert.Equal(t, []string{f3}, event.files)
}

// creates a new file with random name, notifies `watcher`, and returns the file's name
func writeEvent(t *testing.T, watcher watch.Notify, td *testutils.TempDirFixture) string {
	f, err := td.NewFile()
	if err != nil {
		t.Fatal(err)
	}

	watcher.Events() <- fsnotify.Event{Name: f.Name()}

	return f.Name()
}

type serviceWatcherTestFixture struct {
	watcherMaker watcherMaker
	ctx          context.Context
	tempDirs     []*testutils.TempDirFixture
	mounts       []model.Mount
	watchers     []*fakeNotify
	timerMaker   fakeTimerMaker
}

func makeServiceWatcherTestFixture(t *testing.T, mountCount int) *serviceWatcherTestFixture {
	var watchers []*fakeNotify
	nextWatcher := 0
	watcherMaker := func() (watch.Notify, error) {
		if nextWatcher > len(watchers) {
			t.Fatal("tried to get too many watchers")
		}
		ret := watchers[nextWatcher]
		nextWatcher++
		return ret, nil
	}

	var mounts []model.Mount
	var tempDirs []*testutils.TempDirFixture
	for i := 0; i < mountCount; i++ {
		tempDir := testutils.NewTempDirFixture(t)
		mounts = append(mounts, model.Mount{Repo: model.LocalGithubRepo{LocalPath: tempDir.Path()}, ContainerPath: ""})
		tempDirs = append(tempDirs, tempDir)
		watcher := newFakeNotify()
		watchers = append(watchers, watcher)
	}

	timerMaker := makeFakeTimerMaker(t)

	ctx := testutils.CtxForTest()
	return &serviceWatcherTestFixture{watcherMaker, ctx, tempDirs, mounts, watchers, timerMaker}
}

func (s *serviceWatcherTestFixture) TearDown() {
	for _, tempDir := range s.tempDirs {
		tempDir.TearDown()
	}
}
