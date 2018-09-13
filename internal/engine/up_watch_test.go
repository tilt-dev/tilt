package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestServiceWatcher(t *testing.T) {
	tf := makeServiceWatcherTestFixture(t, 1)

	f1 := tf.WriteFile(0)

	tf.AssertNextEvent(0, []string{f1})
}

func TestServiceWatcherTwoServices(t *testing.T) {
	tf := makeServiceWatcherTestFixture(t, 2)

	f1 := tf.WriteFile(0)

	tf.AssertNextEvent(0, []string{f1})

	tf.FreezeTimer()
	f2 := tf.WriteFile(1)
	f3 := tf.WriteFile(0)
	f4 := tf.WriteFile(1)
	tf.UnfreezeTimer()

	tf.AssertNextEvents([]testServiceFilesChangedEvent{
		{1, []string{f2, f4}},
		{0, []string{f3}}})
}

func TestServiceWatcherTwoServicesErr(t *testing.T) {
	tf := makeServiceWatcherTestFixture(t, 2)

	tf.WriteError(1)
	err := tf.Error()
	assert.Error(t, err)
}

type serviceWatcherTestFixture struct {
	sw              *serviceWatcher
	watcherMaker    watcherMaker
	ctx             context.Context
	tempDirs        []*tempdir.TempDirFixture
	services        []model.Manifest
	watchers        []*fakeNotify
	timerMaker      fakeTimerMaker
	t               *testing.T
	numFilesWritten int
}

func makeServiceWatcherTestFixture(t *testing.T, serviceCount int) *serviceWatcherTestFixture {
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

	var services []model.Manifest
	var tempDirs []*tempdir.TempDirFixture
	for i := 0; i < serviceCount; i++ {
		tempDir := tempdir.NewTempDirFixture(t)
		services = append(services,
			model.Manifest{
				Name:   model.ManifestName(fmt.Sprintf("service%v", i)),
				Mounts: []model.Mount{{Repo: model.LocalGithubRepo{LocalPath: tempDir.Path()}, ContainerPath: ""}}})

		tempDirs = append(tempDirs, tempDir)
		watcher := newFakeNotify()
		watchers = append(watchers, watcher)
	}

	timerMaker := makeFakeTimerMaker(t)

	ctx := output.CtxForTest()

	sw, err := makeServiceWatcher(ctx, watcherMaker, timerMaker.maker(), services)

	timerMaker.maxTimerLock.Lock()

	if err != nil {
		t.Fatal(err)
	}

	return &serviceWatcherTestFixture{sw, watcherMaker, ctx, tempDirs, services, watchers, timerMaker, t, 0}
}

func (s *serviceWatcherTestFixture) WriteFile(serviceNumber int) string {
	s.numFilesWritten++
	f, err := s.tempDirs[serviceNumber].NewFile(fmt.Sprintf("f%v_", s.numFilesWritten))
	if err != nil {
		s.t.Fatal(err)
	}

	s.watchers[serviceNumber].Events() <- watch.FileEvent{Path: f.Name()}

	return filepath.Base(f.Name())
}

type testServiceFilesChangedEvent struct {
	serviceNumber int
	files         []string
}

func (s *serviceWatcherTestFixture) readEvents(numExpectedEvents int) []testServiceFilesChangedEvent {
	var ret []testServiceFilesChangedEvent
	for i := 0; i < numExpectedEvents; i++ {
		e := <-s.sw.events
		serviceNumber := -1
		for i := 0; i < len(s.services); i++ {
			if s.services[i].Name == e.service.Name {
				serviceNumber = i
			}
		}
		if serviceNumber == -1 {
			s.t.Fatalf("got event for unknown service %v", e.service)
		}

		var fileBaseNames []string
		for _, f := range e.files {
			fileBaseNames = append(fileBaseNames, filepath.Base(f))
		}
		ret = append(ret, testServiceFilesChangedEvent{serviceNumber, fileBaseNames})
	}

	return ret
}

func (s *serviceWatcherTestFixture) AssertNextEvent(serviceNumber int, files []string) bool {
	return s.AssertNextEvents([]testServiceFilesChangedEvent{{serviceNumber, files}})
}

func (s *serviceWatcherTestFixture) AssertNextEvents(expectedEvents []testServiceFilesChangedEvent) bool {
	actualEvents := s.readEvents(len(expectedEvents))
	return assert.ElementsMatch(s.t, expectedEvents, actualEvents)
}

func (s *serviceWatcherTestFixture) WriteError(serviceNumber int) {
	s.watchers[serviceNumber].errors <- errors.New("test error")
}

func (s *serviceWatcherTestFixture) Error() error {
	return <-s.sw.errs
}

func (s *serviceWatcherTestFixture) FreezeTimer() {
	s.timerMaker.restTimerLock.Lock()
}

func (s *serviceWatcherTestFixture) UnfreezeTimer() {
	s.timerMaker.restTimerLock.Unlock()
}

func (s *serviceWatcherTestFixture) TearDown() {
	for _, tempDir := range s.tempDirs {
		tempDir.TearDown()
	}
}
