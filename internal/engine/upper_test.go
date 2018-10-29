package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/logger"

	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/tiltfile"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	testoutput "github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/watch"
)

const (
	simpleTiltfile = `def foobar():
  start_fast_build("Dockerfile", "docker-tag")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)`
	testContainer = "myTestContainer"
)

// represents a single call to `BuildAndDeploy`
type buildAndDeployCall struct {
	manifest model.Manifest
	state    store.BuildState
}

type fakeBuildAndDeployer struct {
	t     *testing.T
	calls chan buildAndDeployCall

	buildCount int

	// where we store container info for each manifest
	deployInfo map[docker.ImgNameAndTag]container.ID

	// Set this to simulate the build failing. Do not set this directly, use fixture.SetNextBuildFailure
	nextBuildFailure error
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) nextBuildResult(ref reference.Named) store.BuildResult {
	b.buildCount++
	nt, _ := reference.WithTag(ref, fmt.Sprintf("tilt-%d", b.buildCount))
	return store.BuildResult{Image: nt}
}

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (store.BuildResult, error) {
	select {
	case b.calls <- buildAndDeployCall{manifest, state}:
	default:
		b.t.Error("writing to fakeBuildAndDeployer would block. either there's a bug or the buffer size needs to be increased")
	}

	logger.Get(ctx).Infof("fake building %s", manifest.Name)

	err := b.nextBuildFailure
	if err != nil {
		b.nextBuildFailure = nil
		return store.BuildResult{}, err
	}

	return b.nextBuildResult(manifest.DockerRef()), nil
}

func (b *fakeBuildAndDeployer) haveContainerForImage(img reference.NamedTagged) bool {
	_, ok := b.deployInfo[docker.ToImgNameAndTag(img)]
	return ok
}

func (b *fakeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	if result.HasImage() && !b.haveContainerForImage(result.Image) {
		b.deployInfo[docker.ToImgNameAndTag(result.Image)] = container.ID("testcontainer")
	}
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{
		t:          t,
		calls:      make(chan buildAndDeployCall, 5),
		deployInfo: make(map[docker.ImgNameAndTag]container.ID),
	}
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
	defer f.TearDown()
	manifest := f.newManifest("foobar", nil)

	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, false)
	close(f.b.calls)
	assert.Nil(t, err)
	var startedManifests []model.Manifest
	for call := range f.b.calls {
		startedManifests = append(startedManifests, call.manifest)
	}
	assert.Equal(t, []model.Manifest{manifest}, startedManifests)

	state := f.upper.store.RLockState()
	defer f.upper.store.RUnlockState()
	lines := strings.Split(state.ManifestStates[manifest.Name].LastBuildLog.String(), "\n")
	assert.Contains(t, lines, "fake building foobar")
}

func TestUpper_UpWatchError(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	go func() {
		f.fsWatcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)

	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
}

// we can't have a test for a file change w/o error because Up doesn't return unless there's an error
func TestUpper_UpWatchFileChangeThenError(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	go func() {
		f.timerMaker.maxTimerLock.Lock()
		call := <-f.b.calls
		assert.Equal(t, manifest, call.manifest)
		assert.Equal(t, []string{}, call.state.FilesChanged())
		fileRelPath := "fdas"
		f.fsWatcher.events <- watch.FileEvent{Path: fileRelPath}
		call = <-f.b.calls
		assert.Equal(t, manifest, call.manifest)
		assert.Equal(t, "docker.io/library/foobar:tilt-1", call.state.LastImage().String())
		fileAbsPath, err := filepath.Abs(fileRelPath)
		if err != nil {
			t.Errorf("error making abs path of %v: %v", fileRelPath, err)
		}
		assert.Equal(t, []string{fileAbsPath}, call.state.FilesChanged())
		f.fsWatcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChanges(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	go func() {
		f.timerMaker.maxTimerLock.Lock()
		call := <-f.b.calls
		assert.Equal(t, manifest, call.manifest)
		assert.Equal(t, []string{}, call.state.FilesChanged())

		f.timerMaker.restTimerLock.Lock()
		fileRelPaths := []string{"fdas", "giueheh"}
		for _, fileRelPath := range fileRelPaths {
			f.fsWatcher.events <- watch.FileEvent{Path: fileRelPath}
		}
		f.timerMaker.restTimerLock.Unlock()

		call = <-f.b.calls
		assert.Equal(t, manifest, call.manifest)

		var fileAbsPaths []string
		for _, fileRelPath := range fileRelPaths {
			fileAbsPath, err := filepath.Abs(fileRelPath)
			if err != nil {
				t.Errorf("error making abs path of %v: %v", fileRelPath, err)
			}
			fileAbsPaths = append(fileAbsPaths, fileAbsPath)
		}
		assert.Equal(t, fileAbsPaths, call.state.FilesChanged())
		f.fsWatcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}

	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChangesHitMaxTimeout(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	go func() {
		call := <-f.b.calls
		assert.Equal(t, manifest, call.manifest)
		assert.Equal(t, []string{}, call.state.FilesChanged())

		f.timerMaker.maxTimerLock.Lock()
		f.timerMaker.restTimerLock.Lock()
		fileRelPaths := []string{"fdas", "giueheh"}
		for _, fileRelPath := range fileRelPaths {
			f.fsWatcher.events <- watch.FileEvent{Path: fileRelPath}
		}
		f.timerMaker.maxTimerLock.Unlock()

		call = <-f.b.calls
		assert.Equal(t, manifest, call.manifest)

		var fileAbsPaths []string
		for _, fileRelPath := range fileRelPaths {
			fileAbsPath, err := filepath.Abs(fileRelPath)
			if err != nil {
				t.Errorf("error making abs path of %v: %v", fileRelPath, err)
			}
			fileAbsPaths = append(fileAbsPaths, fileAbsPath)
		}
		assert.Equal(t, fileAbsPaths, call.state.FilesChanged())
		f.fsWatcher.errors <- errors.New("bazquu")
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}

	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	f.SetNextBuildFailure(errors.New("Build failed"))
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		f.fsWatcher.events <- watch.FileEvent{Path: "/a.go"}

		call = <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestFirstBuildCancelsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(context.Canceled)

	closeCh := make(chan struct{})
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		close(closeCh)
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, context.Canceled, err)
	<-closeCh
	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileNotWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	buildFailedToken := errors.New("doesn't compile")
	f.SetNextBuildFailure(buildFailedToken)

	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, false)
	expected := fmt.Errorf("Build Failed: %v", buildFailedToken)
	assert.Equal(t, expected, err)
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := <-f.b.calls
	assert.True(t, call.state.IsEmpty())

	// Simulate a change to a.go that makes the build fail.
	f.SetNextBuildFailure(errors.New("Build failed"))
	f.fsWatcher.events <- watch.FileEvent{Path: "/a.go"}

	call = <-f.b.calls
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.state.LastImage().String())
	assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.FileEvent{Path: "/b.go"}

	// The next build should treat both a.go and b.go as changed, and build
	// on the last successful result, from before a.go changed.
	call = <-f.b.calls
	assert.Equal(t, []string{"/a.go", "/b.go"}, call.state.FilesChanged())
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.state.LastImage().String())

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}

	f.assertAllBuildsConsumed()
}

func TestRebuildWithSpuriousChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		// Simulate a change to .#a.go that's a broken symlink.
		realPath := filepath.Join(f.Path(), "a.go")
		tmpPath := filepath.Join(f.Path(), ".#a.go")
		_ = os.Symlink(realPath, tmpPath)

		f.fsWatcher.events <- watch.FileEvent{Path: tmpPath}

		select {
		case <-f.b.calls:
			t.Fatal("Expected to skip build")
		case <-time.After(5 * time.Millisecond):
		}

		f.TouchFiles([]string{realPath})
		f.fsWatcher.events <- watch.FileEvent{Path: realPath}

		call = <-f.b.calls
		assert.Equal(t, []string{tmpPath, realPath}, call.state.FilesChanged())

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestRebuildDockerfileViaImageBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	manifest.ConfigFiles = []string{
		f.JoinPath("Dockerfile"),
	}
	endToken := errors.New("my-err-token")

	// everything that we want to do while watch loop is running
	go func() {
		// First call: with the old manifest
		call := <-f.b.calls
		assert.Empty(t, call.manifest.BaseDockerfile)

		f.WriteFile("Dockerfile", `FROM iron/go:dev`)
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Dockerfile")}

		// Second call: new manifest!
		call = <-f.b.calls
		assert.Equal(t, "FROM iron/go:dev", call.manifest.BaseDockerfile)
		assert.Equal(t, "yaaaaaaaaml", call.manifest.K8sYAML())

		// Since the manifest changed, we cleared the previous build state to force an image build
		assert.False(t, call.state.HasImage())

		f.WriteFile("Tiltfile", simpleTiltfile)
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("random_file.go")}

		// third call: new manifest should persist
		call = <-f.b.calls
		assert.Equal(t, "FROM iron/go:dev", call.manifest.BaseDockerfile)

		// Unchanged manifest --> we do NOT clear the build state
		assert.True(t, call.state.HasImage())

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestMultipleChangesOnlyDeployOneManifest(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def foobar():
  start_fast_build("Dockerfile1", "docker-tag1")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)

  def bazqux():
    start_fast_build("Dockerfile2", "docker-tag2")
    image = stop_build()
    return k8s_service("yaaaaaaaaml", image)
`)
	f.WriteFile("Dockerfile1", `FROM iron/go:dev1`)
	f.WriteFile("Dockerfile2", `FROM iron/go:dev2`)

	mount1 := model.Mount{LocalPath: f.JoinPath("mount1"), ContainerPath: "/go"}
	mount2 := model.Mount{LocalPath: f.JoinPath("mount2"), ContainerPath: "/go"}
	manifest1 := f.newManifest("foobar", []model.Mount{mount1})
	manifest1.ConfigFiles = []string{
		f.JoinPath("mount1", "Dockerfile1"),
	}
	manifest2 := f.newManifest("bazqux", []model.Mount{mount2})
	manifest2.ConfigFiles = []string{
		f.JoinPath("mount2", "Dockerfile2"),
	}

	f.Start([]model.Manifest{manifest1, manifest2}, true)

	// First call: with the old manifests
	call := <-f.b.calls
	assert.Empty(t, call.manifest.BaseDockerfile)
	assert.Equal(t, "foobar", string(call.manifest.Name))

	call = <-f.b.calls
	assert.Empty(t, call.manifest.BaseDockerfile)
	assert.Equal(t, "bazqux", string(call.manifest.Name))

	f.WriteFile("Dockerfile1", `FROM node:10`)
	f.store.Dispatch(manifestFilesChangedAction{
		files:        []string{f.JoinPath("mount1", "Dockerfile1"), f.JoinPath("mount1", "random_file.go")},
		manifestName: manifest1.Name})

	// Second call: one new manifest!
	call = <-f.b.calls

	assert.Equal(t, "foobar", string(call.manifest.Name))
	assert.ElementsMatch(t, []string{f.JoinPath("mount1", "Dockerfile1"), f.JoinPath("mount1", "random_file.go")}, call.state.FilesChanged())

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.state.HasImage())

	f.WaitUntil("all builds complete", func(es store.EngineState) bool {
		return es.CurrentlyBuilding == ""
	})

	// Importantly the other manifest, bazqux, is _not_ called
	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestNoOpChangeToDockerfile(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def foobar():
  start_fast_build("Dockerfile", "docker-tag1")
  add(local_git_repo('.'), '.')
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)

	manifest := f.loadManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	// First call: with the old manifests
	call := <-f.b.calls
	assert.Equal(t, "FROM iron/go:dev1", call.manifest.BaseDockerfile)
	assert.Equal(t, "foobar", string(call.manifest.Name))

	f.store.Dispatch(manifestFilesChangedAction{
		files:        []string{f.JoinPath("Dockerfile"), f.JoinPath("random_file.go")},
		manifestName: manifest.Name,
	})

	// Second call: Editing the Dockerfile means we have to reevaluate the Tiltfile.
	// Editing the random file means we have to do a rebuild. BUT! The Dockerfile
	// hasn't changed, so the manifest hasn't changed, so we can do an incremental build.
	call = <-f.b.calls
	assert.Equal(t, "foobar", string(call.manifest.Name))
	assert.ElementsMatch(t, []string{
		f.JoinPath("Dockerfile"),
		f.JoinPath("random_file.go"),
	}, call.state.FilesChanged())

	// Unchanged manifest --> we do NOT clear the build state
	assert.True(t, call.state.HasImage())

	f.WaitUntil("all builds complete", func(es store.EngineState) bool {
		return es.CurrentlyBuilding == ""
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()

	assert.Contains(t, strings.Join(f.LogLines(), "\n"), "manifest foobar hasn't changed")
}

func TestRebuildDockerfileFailed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	manifest.ConfigFiles = []string{
		f.JoinPath("Tiltfile"),
	}

	f.Start([]model.Manifest{manifest}, true)

	// First call: with the old manifest
	call := <-f.b.calls
	assert.Empty(t, call.manifest.BaseDockerfile)

	// second call: do some stuff
	f.WriteFile("Tiltfile", simpleTiltfile)

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Tiltfile")}
	call = <-f.b.calls
	assert.Equal(t, "FROM iron/go:dev", call.manifest.BaseDockerfile)
	assert.False(t, call.state.HasImage()) // we cleared the previous build state to force an image build

	// Third call: error!
	f.WriteFile("Tiltfile", "borken")
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Tiltfile")}
	select {
	case call := <-f.b.calls:
		t.Errorf("Expected build to not get called, but it did: %+v", call)
	case <-time.After(100 * time.Millisecond):
	}

	// fourth call: fix
	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev2`)

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Dockerfile")}
	call = <-f.b.calls
	assert.Equal(t, "FROM iron/go:dev2", call.manifest.BaseDockerfile)
	assert.False(t, call.state.HasImage()) // we cleared the previous build state to force an image build
	f.WaitUntilManifest("LastError was cleared", "foobar", func(state store.ManifestState) bool {
		return state.LastError == nil
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestBreakAndUnbreakManifestWithNoChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	origTiltfile := `def foobar():
	start_fast_build("Dockerfile", "docker-tag1")
	add(local_git_repo('./nested'), '.')  # Tiltfile is not mounted
	image = stop_build()
	return k8s_service("yaaaaaaaaml", image)`

	f.MkdirAll("nested/.git") // Spoof a git directory -- this is what we'll mount.
	f.WriteFile("Tiltfile", origTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)

	manifest := f.loadManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	// First call: all is well
	_ = <-f.b.calls

	// Second call: change Tiltfile, break manifest
	f.WriteFile("Tiltfile", "borken")
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Tiltfile")}
	select {
	case call := <-f.b.calls:
		t.Errorf("Expected build to not get called, but it did: %+v", call)
	case <-time.After(100 * time.Millisecond):
	}

	// Third call: put Tiltfile back. No change to manifest or to mounted files, so expect no build.
	f.WriteFile("Tiltfile", origTiltfile)

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Tiltfile")}
	select {
	case call := <-f.b.calls:
		t.Errorf("Expected build to not get called, but it did: %+v", call)
	case <-time.After(100 * time.Millisecond):
	}

	f.WaitUntilManifest("LastError was cleared", "foobar", func(state store.ManifestState) bool {
		return state.LastError == nil
	})
}

func TestFilterOutNonMountedConfigFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def foobar():
  start_fast_build("Dockerfile", "docker-tag1")
  add(local_git_repo('./nested'), '.')
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.MkdirAll("nested/.git") // Spoof a git directory -- this is what we'll mount.

	manifest := f.loadManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	// First call: with the old manifests (should be image build)
	call := <-f.b.calls
	assert.False(t, call.state.HasImage()) // No prior build state
	assert.Equal(t, "FROM iron/go:dev", call.manifest.BaseDockerfile)
	assert.Equal(t, "foobar", string(call.manifest.Name))

	f.store.Dispatch(manifestFilesChangedAction{
		files:        []string{f.JoinPath("Dockerfile"), f.JoinPath("nested/random_file.go")},
		manifestName: manifest.Name,
	})

	// Second call: Editing the Dockerfile means we have to reevaluate the Tiltfile, but
	// we made a no-op change --> no change to the manifest, will do an incremental build.
	call = <-f.b.calls
	assert.True(t, call.state.HasImage()) // Had prior build state (i.e. this was an incremental build)
	assert.Equal(t, "foobar", string(call.manifest.Name))

	// 'Dockerfile' didn't get passed through as a changed file b/c it's outside of our mount(s).
	assert.ElementsMatch(t, []string{f.JoinPath("nested/random_file.go")}, call.state.FilesChanged())

	f.WaitUntil("all builds complete", func(es store.EngineState) bool {
		return es.CurrentlyBuilding == ""
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()

	assert.Contains(t, strings.Join(f.LogLines(), "\n"), "manifest foobar hasn't changed")
}

func TestStaticRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar", nil)
	manifest.StaticDockerfile = `FROM golang
ADD ./ ./
go build ./...
`
	manifest.StaticBuildPath = f.Path()

	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		// Simulate a change to main.go
		mainPath := filepath.Join(f.Path(), "main.go")
		f.fsWatcher.events <- watch.FileEvent{Path: mainPath}

		// Check that this triggered a rebuild.
		call = <-f.b.calls
		assert.Equal(t, []string{mainPath}, call.state.FilesChanged())

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestReapOldBuilds(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.docker.BuildCount++
	err := f.upper.reapOldWatchBuilds(f.ctx, []model.Manifest{manifest}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []string{"build-id-0"}, f.docker.RemovedImageIDs)
}

func TestHudUpdated(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	call := <-f.b.calls
	assert.True(t, call.state.IsEmpty())

	err := f.Stop()
	assert.Equal(t, nil, err)

	assert.Equal(t, 1, len(f.hud.LastView.Resources))
	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, manifest.Name, model.ManifestName(rv.Name))
	assert.Equal(t, manifest.Mounts[0].LocalPath, rv.DirectoriesWatched[0])
	f.assertAllBuildsConsumed()
}

func (f *testFixture) testPod(podName string, manifestName string, phase string, cID string, creationTime time.Time) *v1.Pod {
	var containerID string
	if cID != "" {
		containerID = fmt.Sprintf("%s%s", k8s.ContainerIDPrefix, cID)
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              podName,
			CreationTimestamp: metav1.Time{Time: creationTime},
			Labels:            map[string]string{ManifestNameLabel: manifestName},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase(phase),
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:        "test container!",
					Image:       f.imageNameForManifest(manifestName).String(),
					Ready:       true,
					ContainerID: containerID,
				},
			},
		},
	}
}

func setRestartCount(pod *v1.Pod, restartCount int) {
	pod.Status.ContainerStatuses[0].RestartCount = int32(restartCount)
}

func TestPodEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	go func() {
		// Init Action
		<-f.hud.Updates

		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates

		f.podEvent(f.testPod("my pod", "foobar", "CrashLoopBackOff", testContainer, time.Now()))

		<-f.hud.Updates
		<-f.hud.Updates
		rv := f.hud.LastView.Resources[0]
		assert.Equal(t, "my pod", rv.PodName)
		assert.Equal(t, "CrashLoopBackOff", rv.PodStatus)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	time.Sleep(1 * time.Second)
	f.assertAllBuildsConsumed()
}

func TestPodEventContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	var ref reference.NamedTagged
	f.WaitUntilManifest("image appears", "foobar", func(ms store.ManifestState) bool {
		ref = ms.LastBuild.Image
		return ref != nil
	})

	pod := f.testPod("my-pod", "foobar", "Running", testContainer, time.Now())
	pod.Status = k8s.FakePodStatus(ref, "Running")
	pod.Status.ContainerStatuses[0].ContainerID = ""
	pod.Spec = k8s.FakePodSpec(ref)

	f.podEvent(pod)

	podState := store.Pod{}
	f.WaitUntilManifest("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.Pod
		return podState.PodID == "my-pod"
	})

	assert.Equal(t, "", string(podState.ContainerID))
	assert.Equal(t, "main", string(podState.ContainerName))
	assert.Equal(t, []int32{8080}, podState.ContainerPorts)

	err := f.Stop()
	assert.Nil(t, err)
}

func TestPodUnexpectedContainerStartsImageBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)

	// Start and end a fake build to set manifestState.ExpectedContainerId
	f.store.Dispatch(manifestFilesChangedAction{
		manifestName: manifest.Name,
		files:        []string{"/go/a"},
	})
	f.WaitUntil("waiting for manifestToBuild > 0", func(st store.EngineState) bool {
		return len(st.ManifestsToBuild) > 0
	})
	f.store.Dispatch(BuildStartedAction{
		Manifest:  manifest,
		StartTime: time.Now(),
	})
	f.store.Dispatch(BuildCompleteAction{
		Result: store.BuildResult{
			Image:       nil,
			ContainerID: "theOriginalContainer",
		},
	})

	f.podEvent(f.testPod("mypod", "foobar", "Running", "myfunnycontainerid", time.Now()))

	f.WaitUntilManifest("CrashRebuildInProg set to True", "foobar", func(state store.ManifestState) bool {
		return state.CrashRebuildInProg
	})
	// wait for triggered image build (count is 1 because our fake build above doesn't increment this number).
	f.waitForCompletedBuildCount(1)
}

func TestPodEventUpdateByTimestamp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	f.SetNextBuildFailure(errors.New("Build failed"))
	go func() {
		// Init Action
		<-f.hud.Updates

		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates
		<-f.hud.Updates

		firstCreationTime := time.Now()

		f.podEvent(f.testPod("my pod", "foobar", "CrashLoopBackOff", testContainer, firstCreationTime))
		<-f.hud.Updates

		f.podEvent(f.testPod("my new pod", "foobar", "Running", testContainer, firstCreationTime.Add(time.Minute*2)))

		<-f.hud.Updates
		rv := f.hud.LastView.Resources[0]
		assert.Equal(t, "my new pod", rv.PodName)
		assert.Equal(t, "Running", rv.PodStatus)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestPodEventUpdateByPodName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(errors.New("Build failed"))
	f.Start([]model.Manifest{manifest}, true)

	call := <-f.b.calls
	assert.True(t, call.state.IsEmpty())

	f.waitForCompletedBuildCount(1)

	creationTime := time.Now()
	f.podEvent(f.testPod("my pod", "foobar", "CrashLoopBackOff", testContainer, creationTime))

	f.WaitUntil("pod crashes", func(store.EngineState) bool {
		rv := f.hud.LastView.Resources[0]
		return rv.PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my pod", "foobar", "Running", testContainer, creationTime))

	f.WaitUntil("pod comes back", func(store.EngineState) bool {
		rv := f.hud.LastView.Resources[0]
		return rv.PodStatus == "Running"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my pod", rv.PodName)
	assert.Equal(t, "Running", rv.PodStatus)

	err := f.Stop()
	if err != nil {
		t.Fatal(err)
	}

	f.assertAllBuildsConsumed()
}

func TestPodEventIgnoreOlderPod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	f.SetNextBuildFailure(errors.New("Build failed"))
	go func() {
		// Init Action
		<-f.hud.Updates

		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates

		creationTime := time.Now()
		f.podEvent(f.testPod("my new pod", "foobar", "CrashLoopBackOff", testContainer, creationTime))
		<-f.hud.Updates

		f.podEvent(f.testPod("my pod", "foobar", "Running", testContainer, creationTime.Add(time.Minute*-1)))
		<-f.hud.Updates

		rv := f.hud.LastView.Resources[0]
		assert.Equal(t, "my new pod", rv.PodName)
		assert.Equal(t, "CrashLoopBackOff", rv.PodStatus)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestPodContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	<-f.b.calls

	var ref reference.NamedTagged
	f.WaitUntilManifest("image appears", "fe", func(ms store.ManifestState) bool {
		ref = ms.LastBuild.Image
		return ref != nil
	})

	startedAt := time.Now()
	f.podEvent(f.testPod("pod-id", "fe", "Running", testContainer, startedAt))
	f.WaitUntilManifest("pod appears", "fe", func(ms store.ManifestState) bool {
		return ms.Pod.PodID == "pod-id"
	})

	pod := f.testPod("pod-id", "fe", "Running", testContainer, startedAt)
	pod.Spec = k8s.FakePodSpec(ref)
	pod.Status = k8s.FakePodStatus(ref, "Running")
	f.podEvent(pod)

	f.WaitUntilManifest("container is ready", "fe", func(ms store.ManifestState) bool {
		ports := ms.Pod.ContainerPorts
		return len(ports) == 1 && ports[0] == 8080
	})

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}

	f.assertAllBuildsConsumed()
}

func TestUpper_WatchDockerIgnoredFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	manifest.Repos = []model.LocalGithubRepo{
		{
			LocalPath:            f.Path(),
			DockerignoreContents: "dignore.txt",
		},
	}

	go func() {
		call := <-f.b.calls
		assert.Equal(t, manifest, call.manifest)

		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("dignore.txt")}
		time.Sleep(10 * time.Millisecond)
		f.fsWatcher.errors <- errors.New("done")
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	if assert.NotNil(t, err) {
		assert.Equal(t, "done", err.Error())
	}
	f.assertAllBuildsConsumed()
}

func TestUpper_WatchGitIgnoredFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	manifest.Repos = []model.LocalGithubRepo{
		{
			LocalPath:         f.Path(),
			GitignoreContents: "gignore.txt",
		},
	}

	go func() {
		call := <-f.b.calls
		assert.Equal(t, manifest, call.manifest)

		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("gignore.txt")}
		time.Sleep(10 * time.Millisecond)
		f.fsWatcher.errors <- errors.New("done")
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	if assert.NotNil(t, err) {
		assert.Equal(t, "done", err.Error())
	}
	f.assertAllBuildsConsumed()
}

func makeFakeFsWatcherMaker(fn *fakeNotify) FsWatcherMaker {
	return func() (watch.Notify, error) {
		return fn, nil
	}
}

func TestUpper_ShowErrorBuildLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.SetNextBuildFailure(errors.New("failed!"))

	f.Start([]model.Manifest{manifest}, true)

	f.waitForCompletedBuildCount(1)

	f.upper.store.Dispatch(hud.NewShowErrorAction(1))

	f.waitForBuildErrorReplay(manifest.Name,
		`──┤ Building: foobar ├──────────────────────────────────────────────
fake building foobar
`)

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
}

func TestUpper_ShowErrorPodLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(name)
	f.podLog(name, "first string")

	f.upper.store.Dispatch(manifestFilesChangedAction{
		manifestName: "foobar",
		files:        []string{"/go/a.go"},
	})

	f.waitForCompletedBuildCount(2)
	f.podLog(name, "second string")

	f.upper.store.Dispatch(hud.NewShowErrorAction(1))
	f.waitForPodErrorReplay(name, "second string\n")

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
}

func TestUpper_ShowErrorNonExistentResource(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.upper.store.Dispatch(hud.NewShowErrorAction(5))

	f.WaitUntilManifest("nonexistent resource error printed", string(manifest.Name), func(state store.ManifestState) bool {
		for _, s := range f.LogLines() {
			if strings.Contains(s, "Resource 5 does not exist, so no log to print") {
				return true
			}
		}
		return false
	})
	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
}

func TestUpperPodLogInCrashLoopThirdInstanceStillUp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(name)
	f.podLog(name, "first string")
	f.restartPod()
	f.podLog(name, "second string")
	f.restartPod()
	f.podLog(name, "third string")

	f.upper.store.Dispatch(hud.NewShowErrorAction(1))

	// the third instance is still up, so we want to show the log from the last crashed pod plus the log from the current pod
	f.waitForPodErrorReplay(name, "second string\nthird string\n")

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
}

func TestUpperPodLogInCrashLoopPodCurrentlyDown(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(name)
	f.podLog(name, "first string")
	f.restartPod()
	f.podLog(name, "second string")
	f.pod.Status.ContainerStatuses[0].Ready = false
	f.notifyAndWaitForPodStatus(func(pod store.Pod) bool {
		return !pod.ContainerReady
	})

	f.upper.store.Dispatch(hud.NewShowErrorAction(1))

	// The second instance is down, so we don't include the first instance's log
	f.waitForPodErrorReplay(name, "second string\n")

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
}

func testService(serviceName string, manifestName string, ip string, port int) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: map[string]string{ManifestNameLabel: manifestName},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Port: int32(port),
			}},
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: ip,
					},
				},
			},
		},
	}
}

func TestUpper_ServiceEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.upper.store.Dispatch(NewServiceChangeAction(testService("myservice", "foobar", "1.2.3.4", 8080)))

	f.WaitUntilManifest("lb updated", "foobar", func(ms store.ManifestState) bool {
		return len(ms.LBs) > 0
	})

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}

	ms := f.upper.store.RLockState().ManifestStates[manifest.Name]
	defer f.upper.store.RUnlockState()
	assert.Equal(t, 1, len(ms.LBs))
	url, ok := ms.LBs["myservice"]
	if !ok {
		t.Fatalf("%v did not contain key 'myservice'", ms.LBs)
	}
	assert.Equal(t, "http://1.2.3.4:8080/", url.String())
}

func TestUpper_PodLogs(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(name)

	f.podLog(name, "Hello world!\n")

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
}

func TestCancelingUpperCancelsHud(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, f.hud.Canceled)
}

func TestCompletingUpperClosesHud(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, false)
	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, f.hud.Closed)
}

func TestInitWithGlobalYAML(t *testing.T) {
	f := newTestFixture(t)
	state := f.store.RLockState()
	ym := model.NewYAMLManifest(model.ManifestName("global"), "this is the global yaml", []string{})
	state.GlobalYAML = ym
	f.store.RUnlockState()

	f.Start([]model.Manifest{}, true)

	f.store.Dispatch(InitAction{
		Manifests:          []model.Manifest{},
		GlobalYAMLManifest: ym,
	})

	f.WaitUntil("huh", func(st store.EngineState) bool {
		return len(st.ManifestsToBuild) > 0
	})
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

func makeFakePodWatcherMaker(ch chan *v1.Pod) func(context.Context, *store.Store) error {
	return func(ctx context.Context, st *store.Store) error {
		go dispatchPodChangesLoop(ctx, ch, st)
		return nil
	}
}

func makeFakeServiceWatcherMaker(ch chan *v1.Service) func(context.Context, *store.Store) error {
	return func(ctx context.Context, st *store.Store) error {
		go dispatchServiceChangesLoop(ctx, ch, st)
		return nil
	}
}

type testFixture struct {
	*tempdir.TempDirFixture
	ctx                   context.Context
	cancel                func()
	upper                 Upper
	b                     *fakeBuildAndDeployer
	fsWatcher             *fakeNotify
	timerMaker            *fakeTimerMaker
	docker                *docker.FakeDockerClient
	hud                   *hud.FakeHud
	createManifestsResult chan error
	log                   *bufsync.ThreadSafeBuffer
	store                 *store.Store
	pod                   *v1.Pod
	bc                    *BuildController
}

func newTestFixture(t *testing.T) *testFixture {
	f := tempdir.NewTempDirFixture(t)
	watcher := newFakeNotify()
	b := newFakeBuildAndDeployer(t)

	timerMaker := makeFakeTimerMaker(t)

	docker := docker.NewFakeDockerClient()
	reaper := build.NewImageReaper(docker)

	k8s := k8s.NewFakeK8sClient()
	pw := NewPodWatcher(k8s)
	sw := NewServiceWatcher(k8s)

	hud := hud.NewFakeHud()

	log := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(testoutput.ForkedCtxForTest(log))

	st := store.NewStore()

	plm := NewPodLogManager(k8s)
	bc := NewBuildController(b)

	_ = os.Chdir(f.Path())
	_ = os.Mkdir(f.JoinPath(".git"), os.FileMode(0777))

	fswm := func() (watch.Notify, error) {
		return watcher, nil
	}
	fwm := NewWatchManager(fswm, timerMaker.maker())
	pfc := NewPortForwardController(k8s)

	upper := NewUpper(ctx, b, k8s, reaper, hud, pw, sw, st, plm, pfc, fwm, fswm, bc)
	upper.timerMaker = timerMaker.maker()
	upper.hudErrorCh = make(chan error)

	go func() {
		upper.RunHud(ctx)
	}()

	return &testFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		upper:          upper,
		b:              b,
		fsWatcher:      watcher,
		timerMaker:     &timerMaker,
		docker:         docker,
		hud:            hud,
		log:            log,
		store:          st,
		bc:             bc,
	}
}

func (f *testFixture) Start(manifests []model.Manifest, watchMounts bool) {
	f.createManifestsResult = make(chan error)

	go func() {
		err := f.upper.CreateManifests(f.ctx, manifests, watchMounts)
		if err != nil && err != context.Canceled {
			// Print this out here in case the test never completes
			log.Printf("CreateManifests failed: %v", err)
			f.cancel()
		}
		f.createManifestsResult <- err
	}()

	// Init Action
	<-f.hud.Updates
}

func (f *testFixture) Stop() error {
	f.cancel()
	err := <-f.createManifestsResult
	if err == context.Canceled {
		return nil
	} else {
		return err
	}
}

func (f *testFixture) SetNextBuildFailure(err error) {
	// Don't set the nextBuildFailure flag when a completed build needs to be processed
	// by the state machine.
	f.WaitUntil("build complete processed", func(state store.EngineState) bool {
		return state.CurrentlyBuilding == ""
	})
	_ = f.store.RLockState()
	f.b.nextBuildFailure = err
	f.store.RUnlockState()
}

// Wait until the given engine state test passes.
func (f *testFixture) WaitUntil(msg string, isDone func(store.EngineState) bool) {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	for {
		state := f.upper.store.RLockState()
		done := isDone(state)
		f.upper.store.RUnlockState()
		if done {
			return
		}

		select {
		case <-ctx.Done():
			f.T().Fatalf("Timed out waiting for: %s", msg)
			// TODO(nick): Right now we're using the HUD update channel as a proxy for
			// "the model changed". Eventually we should have a real reactive
			// subscription mechanism.
		case <-f.hud.Updates:
		}
	}
}

func (f *testFixture) WaitUntilManifest(msg string, name string, isDone func(store.ManifestState) bool) {
	f.WaitUntil(msg, func(es store.EngineState) bool {
		ms, ok := es.ManifestStates[model.ManifestName(name)]
		if !ok {
			return false
		}
		return isDone(*ms)
	})
}

func (f *testFixture) startPod(manifestName model.ManifestName) {
	pID := k8s.PodID("mypod")
	f.pod = f.testPod(pID.String(), manifestName.String(), "Running", testContainer, time.Now())
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifest("pod appears", manifestName.String(), func(ms store.ManifestState) bool {
		return ms.Pod.PodID == k8s.PodID(f.pod.Name)
	})
}

func (f *testFixture) podLog(manifestName model.ManifestName, s string) {
	f.upper.store.Dispatch(PodLogAction{
		ManifestName: manifestName,
		PodID:        k8s.PodID(f.pod.Name),
		Log:          []byte(s + "\n"),
	})

	f.WaitUntilManifest("pod log seen", string(manifestName), func(ms store.ManifestState) bool {
		return strings.Contains(string(ms.Pod.CurrentLog), s)
	})
}

func (f *testFixture) restartPod() {
	restartCount := f.pod.Status.ContainerStatuses[0].RestartCount + 1
	f.pod.Status.ContainerStatuses[0].RestartCount = restartCount
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifest("pod restart seen", "foobar", func(ms store.ManifestState) bool {
		return ms.Pod.ContainerRestarts == int(restartCount)
	})
}

func (f *testFixture) notifyAndWaitForPodStatus(pred func(pod store.Pod) bool) {
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifest("pod status change seen", "foobar", func(state store.ManifestState) bool {
		return pred(state.Pod)
	})
}

func (f *testFixture) waitForBuildErrorReplay(name model.ManifestName, message string) {
	f.WaitUntil("build log shown", func(s store.EngineState) bool {
		expectedOutput := strings.Join([]string{
			fmt.Sprintf("Last %s build log:", name),
			"──────────────────────────────────────────────────────────",
			message,
			"──────────────────────────────────────────────────────────",
		}, "\n")
		return strings.Contains(f.log.String(), expectedOutput)
	})
}

func (f *testFixture) waitForPodErrorReplay(name model.ManifestName, message string) {
	f.WaitUntil("pod log shown", func(s store.EngineState) bool {
		expectedOutput := strings.Join([]string{
			fmt.Sprintf("%s pod log:", name),
			"──────────────────────────────────────────────────────────",
			message,
			"──────────────────────────────────────────────────────────",
		}, "\n")
		return strings.Contains(f.log.String(), expectedOutput)
	})
}

func (f *testFixture) waitForCompletedBuildCount(count int) {
	f.WaitUntil(fmt.Sprintf("%d builds done", count), func(state store.EngineState) bool {
		return state.CompletedBuildCount == count
	})
}

func (f *testFixture) LogLines() []string {
	return strings.Split(f.log.String(), "\n")
}

func (f *testFixture) TearDown() {
	f.TempDirFixture.TearDown()
	f.cancel()
}

func (f *testFixture) podEvent(pod *v1.Pod) {
	f.store.Dispatch(NewPodChangeAction(pod))
}

func (f *testFixture) imageNameForManifest(manifestName string) reference.Named {
	ref, err := reference.ParseNormalizedNamed(manifestName)
	if err != nil {
		f.T().Fatal(err)
	}
	return ref
}

func (f *testFixture) newManifest(name string, mounts []model.Mount) model.Manifest {
	ref := f.imageNameForManifest(name)
	return model.Manifest{Name: model.ManifestName(name), Mounts: mounts}.WithDockerRef(ref)
}

func (f *testFixture) assertAllBuildsConsumed() {
	close(f.b.calls)

	for call := range f.b.calls {
		f.T().Fatalf("Build not consumed: %+v", call)
	}
}

func (f *testFixture) consumeAllHudUpdates() {
	done := false
	for !done {
		select {
		case <-f.hud.Updates:
		default:
			done = true
		}
	}
}

func (f *testFixture) loadManifest(name string) model.Manifest {
	tf, err := tiltfile.Load(f.ctx, f.JoinPath("Tiltfile"))
	if err != nil {
		f.T().Fatal(err)
	}
	manifests, _, err := tf.GetManifestConfigsAndGlobalYAML("foobar")
	if err != nil {
		f.T().Fatal(err)
	}
	assert.Equal(f.T(), 1, len(manifests))
	return manifests[0]
}
