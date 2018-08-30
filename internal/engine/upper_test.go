package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/watch"
)

//represents a single call to `BuildAndDeploy`
type buildAndDeployCall struct {
	service model.Service
	state   BuildState
}

type fakeBuildAndDeployer struct {
	t     *testing.T
	calls chan buildAndDeployCall

	buildCount int

	// Set this to simulate the build failing
	nextBuildFailure error
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) nextBuildResult() BuildResult {
	b.buildCount++
	n, _ := reference.WithName("windmill.build/dummy")
	nt, _ := reference.WithTag(n, fmt.Sprintf("tilt-%d", b.buildCount))
	return BuildResult{Image: nt}
}

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	select {
	case b.calls <- buildAndDeployCall{service, state}:
	default:
		b.t.Error("writing to fakeBuildAndDeployer would block. either there's a bug or the buffer size needs to be increased")
	}

	err := b.nextBuildFailure
	if err != nil {
		b.nextBuildFailure = nil
		return BuildResult{}, err
	}

	return b.nextBuildResult(), nil
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{t: t, calls: make(chan buildAndDeployCall, 5)}
}

type fakeNotify struct {
	paths  []string
	events chan watch.FileEvent
	errors chan error
}

func (n *fakeNotify) Add(name string) error {
	n.paths = append(n.paths, name)
	return nil
}

func (n *fakeNotify) Close() error {
	close(n.events)
	close(n.errors)
	return nil
}

func (n *fakeNotify) Errors() chan error {
	return n.errors
}

func (n *fakeNotify) Events() chan watch.FileEvent {
	return n.events
}

func newFakeNotify() *fakeNotify {
	return &fakeNotify{paths: make([]string, 0), errors: make(chan error, 1), events: make(chan watch.FileEvent, 10)}
}

var _ watch.Notify = &fakeNotify{}

func TestUpper_Up(t *testing.T) {
	f := newTestFixture(t)
	service := model.Service{Name: "foobar"}
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, false)
	close(f.b.calls)
	assert.Nil(t, err)
	var startedServices []model.Service
	for call := range f.b.calls {
		startedServices = append(startedServices, call.service)
	}
	assert.Equal(t, []model.Service{service}, startedServices)
}

func TestUpper_UpWatchZeroRepos(t *testing.T) {
	f := newTestFixture(t)
	service := model.Service{Name: "foobar"}
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "nothing to watch")
	}
}

func TestUpper_UpWatchError(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	go func() {
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	close(f.b.calls)

	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}

	var startedServices []model.Service
	for call := range f.b.calls {
		startedServices = append(startedServices, call.service)
	}

	assert.Equal(t, []model.Service{service}, startedServices)
}

// we can't have a test for a file change w/o error because Up doesn't return unless there's an error
func TestUpper_UpWatchFileChangeThenError(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	go func() {
		f.timerMaker.maxTimerLock.Lock()
		call := <-f.b.calls
		assert.Equal(t, service, call.service)
		assert.Equal(t, []string{}, call.state.FilesChanged())
		fileRelPath := "fdas"
		f.watcher.events <- watch.FileEvent{Path: fileRelPath}
		call = <-f.b.calls
		assert.Equal(t, service, call.service)
		assert.Equal(t, "windmill.build/dummy:tilt-1", call.state.LastImage().String())
		fileAbsPath, err := filepath.Abs(fileRelPath)
		if err != nil {
			t.Errorf("error making abs path of %v: %v", fileRelPath, err)
		}
		assert.Equal(t, []string{fileAbsPath}, call.state.FilesChanged())
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	close(f.b.calls)

	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
}

func TestUpper_UpWatchCoalescedFileChanges(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	go func() {
		f.timerMaker.maxTimerLock.Lock()
		call := <-f.b.calls
		assert.Equal(t, service, call.service)
		assert.Equal(t, []string{}, call.state.FilesChanged())

		f.timerMaker.restTimerLock.Lock()
		fileRelPaths := []string{"fdas", "giueheh"}
		for _, fileRelPath := range fileRelPaths {
			f.watcher.events <- watch.FileEvent{Path: fileRelPath}
		}
		f.timerMaker.restTimerLock.Unlock()

		call = <-f.b.calls
		assert.Equal(t, service, call.service)

		var fileAbsPaths []string
		for _, fileRelPath := range fileRelPaths {
			fileAbsPath, err := filepath.Abs(fileRelPath)
			if err != nil {
				t.Errorf("error making abs path of %v: %v", fileRelPath, err)
			}
			fileAbsPaths = append(fileAbsPaths, fileAbsPath)
		}
		assert.Equal(t, fileAbsPaths, call.state.FilesChanged())
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	close(f.b.calls)

	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
}

func TestUpper_UpWatchCoalescedFileChangesHitMaxTimeout(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	go func() {
		call := <-f.b.calls
		assert.Equal(t, service, call.service)
		assert.Equal(t, []string{}, call.state.FilesChanged())

		f.timerMaker.maxTimerLock.Lock()
		f.timerMaker.restTimerLock.Lock()
		fileRelPaths := []string{"fdas", "giueheh"}
		for _, fileRelPath := range fileRelPaths {
			f.watcher.events <- watch.FileEvent{Path: fileRelPath}
		}
		f.timerMaker.maxTimerLock.Unlock()

		call = <-f.b.calls
		assert.Equal(t, service, call.service)

		var fileAbsPaths []string
		for _, fileRelPath := range fileRelPaths {
			fileAbsPath, err := filepath.Abs(fileRelPath)
			if err != nil {
				t.Errorf("error making abs path of %v: %v", fileRelPath, err)
			}
			fileAbsPaths = append(fileAbsPaths, fileAbsPath)
		}
		assert.Equal(t, fileAbsPaths, call.state.FilesChanged())
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	close(f.b.calls)

	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		// Simulate a change to a.go that makes the build fail.
		f.b.nextBuildFailure = errors.New("Build failed")
		f.watcher.events <- watch.FileEvent{Path: "/a.go"}

		call = <-f.b.calls
		assert.Equal(t, "windmill.build/dummy:tilt-1", call.state.LastImage().String())
		assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

		// Simulate a change to b.go
		f.watcher.events <- watch.FileEvent{Path: "/b.go"}

		// The next build should treat both a.go and b.go as changed, and build
		// on the last successful result, from before a.go changed.
		call = <-f.b.calls
		assert.Equal(t, []string{"/a.go", "/b.go"}, call.state.FilesChanged())
		assert.Equal(t, "windmill.build/dummy:tilt-1", call.state.LastImage().String())

		f.watcher.errors <- endToken
	}()
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	assert.Equal(t, endToken, err)
}

func TestRebuildWithSpuriousChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		// Simulate a change to .#a.go that's a broken symlink.
		realPath := filepath.Join(f.Path(), "a.go")
		tmpPath := filepath.Join(f.Path(), ".#a.go")
		_ = os.Symlink(realPath, tmpPath)

		f.watcher.events <- watch.FileEvent{Path: tmpPath}

		select {
		case <-f.b.calls:
			t.Fatal("Expected to skip build")
		case <-time.After(5 * time.Millisecond):
		}

		f.TouchFiles([]string{realPath})
		f.watcher.events <- watch.FileEvent{Path: realPath}

		call = <-f.b.calls
		assert.Equal(t, []string{tmpPath, realPath}, call.state.FilesChanged())

		f.watcher.errors <- endToken
	}()
	err := f.upper.CreateServices(f.Ctx(), []model.Service{service}, true)
	assert.Equal(t, endToken, err)
}

type fakeTimerMaker struct {
	restTimerLock *sync.Mutex
	maxTimerLock  *sync.Mutex
	t             *testing.T
}

func (f fakeTimerMaker) maker() timerMaker {
	return func(d time.Duration) <-chan time.Time {
		var lock *sync.Mutex
		// we have separate locks for the separate uses of timer so that tests can control the timers independently
		switch d {
		case watchBufferMinRestDuration:
			lock = f.restTimerLock
		case watchBufferMaxDuration:
			lock = f.maxTimerLock
		default:
			// if you hit this, someone (you!?) might have added a new timer with a new duration, and you probably
			// want to add a case above
			f.t.Error("makeTimer called on unsupported duration")
		}
		ret := make(chan time.Time, 1)
		go func() {
			lock.Lock()
			ret <- time.Unix(0, 0)
			lock.Unlock()
			close(ret)
		}()
		return ret
	}
}

func makeFakeTimerMaker(t *testing.T) fakeTimerMaker {
	restTimerLock := new(sync.Mutex)
	maxTimerLock := new(sync.Mutex)

	return fakeTimerMaker{restTimerLock, maxTimerLock, t}
}

func makeFakeWatcherMaker(fn *fakeNotify) watcherMaker {
	return func() (watch.Notify, error) {
		return fn, nil
	}
}

type testFixture struct {
	*testutils.TempDirFixture
	upper      Upper
	b          *fakeBuildAndDeployer
	watcher    *fakeNotify
	timerMaker *fakeTimerMaker
}

func newTestFixture(t *testing.T) *testFixture {
	f := testutils.NewTempDirFixture(t)
	watcher := newFakeNotify()
	watcherMaker := makeFakeWatcherMaker(watcher)
	b := newFakeBuildAndDeployer(t)

	timerMaker := makeFakeTimerMaker(t)

	k8s := &FakeK8sClient{}
	upper := Upper{b, watcherMaker, timerMaker.maker(), k8s}
	return &testFixture{f, upper, b, watcher, &timerMaker}
}
