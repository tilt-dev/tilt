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

func TestManifestWatcher(t *testing.T) {
	tf := makeManifestWatcherTestFixture(t, 1)

	f1 := tf.WriteFile(0)

	tf.AssertNextEvent(0, []string{f1})
}

func TestManifestWatcherTwoManifests(t *testing.T) {
	tf := makeManifestWatcherTestFixture(t, 2)

	f1 := tf.WriteFile(0)

	tf.AssertNextEvent(0, []string{f1})

	tf.FreezeTimer()
	f2 := tf.WriteFile(1)
	f3 := tf.WriteFile(0)
	f4 := tf.WriteFile(1)
	tf.UnfreezeTimer()

	tf.AssertNextEvents([]testManifestFilesChangedEvent{
		{1, []string{f2, f4}},
		{0, []string{f3}}})
}

func TestManifestWatcherTwoManifestsErr(t *testing.T) {
	tf := makeManifestWatcherTestFixture(t, 2)

	tf.WriteError(1)
	err := tf.Error()
	assert.Error(t, err)
}

type manifestWatcherTestFixture struct {
	sw              *manifestWatcher
	watcherMaker    watcherMaker
	ctx             context.Context
	tempDirs        []*tempdir.TempDirFixture
	manifests       []model.Manifest
	watchers        []*fakeNotify
	timerMaker      fakeTimerMaker
	t               *testing.T
	numFilesWritten int
}

func makeManifestWatcherTestFixture(t *testing.T, manifestCount int) *manifestWatcherTestFixture {
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

	var manifests []model.Manifest
	var tempDirs []*tempdir.TempDirFixture
	for i := 0; i < manifestCount; i++ {
		tempDir := tempdir.NewTempDirFixture(t)
		manifests = append(manifests,
			model.Manifest{
				Name:   model.ManifestName(fmt.Sprintf("manifest%v", i)),
				Mounts: []model.Mount{{Repo: model.LocalGithubRepo{LocalPath: tempDir.Path()}, ContainerPath: ""}}})

		tempDirs = append(tempDirs, tempDir)
		watcher := newFakeNotify()
		watchers = append(watchers, watcher)
	}

	timerMaker := makeFakeTimerMaker(t)

	ctx := output.CtxForTest()

	sw, err := makeManifestWatcher(ctx, watcherMaker, timerMaker.maker(), manifests)

	timerMaker.maxTimerLock.Lock()

	if err != nil {
		t.Fatal(err)
	}

	return &manifestWatcherTestFixture{sw, watcherMaker, ctx, tempDirs, manifests, watchers, timerMaker, t, 0}
}

func (s *manifestWatcherTestFixture) WriteFile(manifestNumber int) string {
	s.numFilesWritten++
	f, err := s.tempDirs[manifestNumber].NewFile(fmt.Sprintf("f%v_", s.numFilesWritten))
	if err != nil {
		s.t.Fatal(err)
	}

	s.watchers[manifestNumber].Events() <- watch.FileEvent{Path: f.Name()}

	return filepath.Base(f.Name())
}

type testManifestFilesChangedEvent struct {
	manifestNumber int
	files          []string
}

func (s *manifestWatcherTestFixture) readEvents(numExpectedEvents int) []testManifestFilesChangedEvent {
	var ret []testManifestFilesChangedEvent
	for i := 0; i < numExpectedEvents; i++ {
		e := <-s.sw.events
		manifestNumber := -1
		for i := 0; i < len(s.manifests); i++ {
			if s.manifests[i].Name == e.manifest.Name {
				manifestNumber = i
			}
		}
		if manifestNumber == -1 {
			s.t.Fatalf("got event for unknown manifest %v", e.manifest)
		}

		var fileBaseNames []string
		for _, f := range e.files {
			fileBaseNames = append(fileBaseNames, filepath.Base(f))
		}
		ret = append(ret, testManifestFilesChangedEvent{manifestNumber, fileBaseNames})
	}

	return ret
}

func (s *manifestWatcherTestFixture) AssertNextEvent(manifestNumber int, files []string) bool {
	return s.AssertNextEvents([]testManifestFilesChangedEvent{{manifestNumber, files}})
}

func (s *manifestWatcherTestFixture) AssertNextEvents(expectedEvents []testManifestFilesChangedEvent) bool {
	actualEvents := s.readEvents(len(expectedEvents))
	return assert.ElementsMatch(s.t, expectedEvents, actualEvents)
}

func (s *manifestWatcherTestFixture) WriteError(manifestNumber int) {
	s.watchers[manifestNumber].errors <- errors.New("test error")
}

func (s *manifestWatcherTestFixture) Error() error {
	return <-s.sw.errs
}

func (s *manifestWatcherTestFixture) FreezeTimer() {
	s.timerMaker.restTimerLock.Lock()
}

func (s *manifestWatcherTestFixture) UnfreezeTimer() {
	s.timerMaker.restTimerLock.Unlock()
}

func (s *manifestWatcherTestFixture) TearDown() {
	for _, tempDir := range s.tempDirs {
		tempDir.TearDown()
	}
}
