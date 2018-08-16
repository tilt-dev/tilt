package engine

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/fsnotify"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

type fakeBuildAndDeployer struct {
	startedServices []model.Service
	calls           chan bool
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token BuildToken) (BuildToken, error) {
	b.startedServices = append(b.startedServices, service)
	b.calls <- true
	return nil, nil
}

func newFakeBuildAndDeployer() *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{calls: make(chan bool, 5)}
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
	assert.Nil(t, err)
	assert.Equal(t, []model.Service{service}, f.b.startedServices)
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
	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
	assert.Equal(t, []model.Service{service}, f.b.startedServices)
}

// we can't have a test for a file change w/o error because Up doesn't return unless there's an error
func TestUpper_UpWatchFileChangeThenError(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{Repo: model.LocalGithubRepo{LocalPath: "/go"}, ContainerPath: "/go"}
	service := model.Service{Name: "foobar", Mounts: []model.Mount{mount}}
	go func() {
		<-f.b.calls
		f.watcher.events <- fsnotify.Event{}
		<-f.b.calls
		f.watcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.Up(f.context, service, true, os.Stdout, os.Stderr)
	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
	//file was touched once, so service should have been deployed twice
	assert.Equal(t, []model.Service{service, service}, f.b.startedServices)
}

type testFixture struct {
	t       *testing.T
	upper   Upper
	b       *fakeBuildAndDeployer
	watcher *fakeNotify
	context context.Context
}

func newTestFixture(t *testing.T) *testFixture {
	watcher := newFakeNotify()
	watcherMaker := func() (watch.Notify, error) {
		return watcher, nil
	}
	b := newFakeBuildAndDeployer()
	upper := Upper{b, watcherMaker}
	ctx := logger.WithLogger(context.Background(), logger.NewLogger(logger.DebugLvl))
	return &testFixture{t, upper, b, watcher, ctx}
}
