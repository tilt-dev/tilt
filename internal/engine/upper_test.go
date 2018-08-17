package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/fsnotify"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

//represents a single call to `BuildAndDeploy`
type buildAndDeployCall struct {
	service model.Service
	files   []string
}

type fakeBuildAndDeployer struct {
	t     *testing.T
	calls chan buildAndDeployCall
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error) {
	select {
	case b.calls <- buildAndDeployCall{service, changedFiles}:
	default:
		b.t.Error("writing to fakeBuildAndDeployer would block. either there's a bug or the buffer size needs to be increased")
	}
	return nil, nil
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{t: t, calls: make(chan buildAndDeployCall, 5)}
}

type fakeNotify struct {
	paths  []string
	events chan fsnotify.Event
	errors chan error
}

func (n *fakeNotify) Add(name string) error {
	n.paths = append(n.paths, name)
	return nil
}

func (n *fakeNotify) Close() error {
	return nil
}

func (n *fakeNotify) Errors() chan error {
	return n.errors
}

func (n *fakeNotify) Events() chan fsnotify.Event {
	return n.events
}

func newFakeNotify() *fakeNotify {
	return &fakeNotify{paths: make([]string, 0), errors: make(chan error, 1), events: make(chan fsnotify.Event, 10)}
}

var _ watch.Notify = &fakeNotify{}

func TestUpper_Up(t *testing.T) {
	f := newTestFixture(t)
	service := model.Service{Name: "foobar"}
	err := f.upper.Up(f.context, service, false, os.Stdout, os.Stderr)
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
	err := f.upper.Up(f.context, service, true, os.Stdout, os.Stderr)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "0 repos")
	}
}

func TestUpper_UpWatchError(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	go func() {
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.Up(f.context, service, true, os.Stdout, os.Stderr)
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
		call := <-f.b.calls
		assert.Equal(t, service, call.service)
		assert.Equal(t, []string(nil), call.files)
		fileRelPath := "fdas"
		f.watcher.events <- fsnotify.Event{Name: fileRelPath}
		call = <-f.b.calls
		assert.Equal(t, service, call.service)
		fileAbsPath, err := filepath.Abs(fileRelPath)
		if err != nil {
			t.Errorf("error making abs path of %v: %v", fileRelPath, err)
		}
		assert.Equal(t, []string{fileAbsPath}, call.files)
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.Up(f.context, service, true, os.Stdout, os.Stderr)
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
		call := <-f.b.calls
		assert.Equal(t, service, call.service)
		assert.Equal(t, []string(nil), call.files)

		f.sleepLock.Lock()
		fileRelPaths := []string{"fdas", "giueheh"}
		for _, fileRelPath := range fileRelPaths {
			f.watcher.events <- fsnotify.Event{Name: fileRelPath}
		}
		f.sleepLock.Unlock()

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
		assert.Equal(t, fileAbsPaths, call.files)
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.Up(f.context, service, true, os.Stdout, os.Stderr)
	close(f.b.calls)

	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
}

type testFixture struct {
	t         *testing.T
	upper     Upper
	b         *fakeBuildAndDeployer
	watcher   *fakeNotify
	context   context.Context
	sleepLock *sync.Mutex
}

func newTestFixture(t *testing.T) *testFixture {
	watcher := newFakeNotify()
	watcherMaker := func() (watch.Notify, error) {
		return watcher, nil
	}
	b := newFakeBuildAndDeployer(t)
	sleepLock := &sync.Mutex{}
	// a sleeper that is blockable by tests via sleepLock.Lock()
	sleeper := func(d time.Duration) { sleepLock.Lock(); sleepLock.Unlock() }
	upper := Upper{b, watcherMaker, sleeper}
	ctx := logger.WithLogger(context.Background(), logger.NewLogger(logger.DebugLvl))
	return &testFixture{t, upper, b, watcher, ctx, sleepLock}
}
