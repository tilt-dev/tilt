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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/api/core/v1"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/watch"
)

// represents a single call to `BuildAndDeploy`
type buildAndDeployCall struct {
	manifest model.Manifest
	state    BuildState
}

type fakeBuildAndDeployer struct {
	t     *testing.T
	calls chan buildAndDeployCall

	buildCount int

	// where we store container info for each manifest
	deployInfo map[docker.ImgNameAndTag]k8s.ContainerID

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

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state BuildState) (BuildResult, error) {
	select {
	case b.calls <- buildAndDeployCall{manifest, state}:
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

func (b *fakeBuildAndDeployer) haveContainerForImage(img reference.NamedTagged) bool {
	_, ok := b.deployInfo[docker.ToImgNameAndTag(img)]
	return ok
}

func (b *fakeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult BuildResult) {
	if result.HasImage() && !b.haveContainerForImage(result.Image) {
		b.deployInfo[docker.ToImgNameAndTag(result.Image)] = k8s.ContainerID("testcontainer")
	}
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{
		t:          t,
		calls:      make(chan buildAndDeployCall, 5),
		deployInfo: make(map[docker.ImgNameAndTag]k8s.ContainerID),
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
}

func TestUpper_UpWatchZeroRepos(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar", nil)
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "nothing to watch")
	}
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
		assert.Equal(t, "windmill.build/dummy:tilt-1", call.state.LastImage().String())
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
	f.b.nextBuildFailure = errors.New("Build failed")
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
	f.b.nextBuildFailure = context.Canceled

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
	f.b.nextBuildFailure = buildFailedToken

	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, false)
	expected := fmt.Errorf("build failed: %v", buildFailedToken)
	assert.Equal(t, expected, err)
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		// Simulate a change to a.go that makes the build fail.
		f.b.nextBuildFailure = errors.New("Build failed")
		f.fsWatcher.events <- watch.FileEvent{Path: "/a.go"}

		call = <-f.b.calls
		assert.Equal(t, "windmill.build/dummy:tilt-1", call.state.LastImage().String())
		assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

		// Simulate a change to b.go
		f.fsWatcher.events <- watch.FileEvent{Path: "/b.go"}

		// The next build should treat both a.go and b.go as changed, and build
		// on the last successful result, from before a.go changed.
		call = <-f.b.calls
		assert.Equal(t, []string{"/a.go", "/b.go"}, call.state.FilesChanged())
		assert.Equal(t, "windmill.build/dummy:tilt-1", call.state.LastImage().String())

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
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

func TestRebuildDockerfile(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	os.Chdir(f.Path())
	f.WriteFile("Tiltfile", `def foobar():
  start_fast_build("Dockerfile", "docker-tag")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)

	mount := model.Mount{LocalPath: f.TempDirFixture.Path(), ContainerPath: "/go"}
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
		assert.Equal(t, "yaaaaaaaaml", call.manifest.K8sYaml)

		f.WriteFile("Tiltfile", `def foobar():
	start_fast_build("Dockerfile", "docker-tag")
	image = stop_build()
	return k8s_service("yaaaaaaaaml", image)
`)
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("random_file.go")}
		// third call: new manifest should persist
		call = <-f.b.calls
		assert.Equal(t, "FROM iron/go:dev", call.manifest.BaseDockerfile)

		f.fsWatcher.errors <- endToken
	}()
	err = f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
}

func TestRebuildDockerfileFailed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	os.Chdir(f.Path())
	f.WriteFile("Tiltfile", `def foobar():
  start_fast_build("Dockerfile", "docker-tag")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)

	mount := model.Mount{LocalPath: f.TempDirFixture.Path(), ContainerPath: "/go"}
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

		// second call: do some stuff
		f.WriteFile("Tiltfile", `def foobar():
	start_fast_build("Dockerfile", "docker-tag")
	image = stop_build()
	return k8s_service("yaaaaaaaaml", image)
`)

		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Dockerfile")}
		call = <-f.b.calls
		assert.Equal(t, "FROM iron/go:dev", call.manifest.BaseDockerfile)

		// Third call: error!
		f.WriteFile("Tiltfile", "def")
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Dockerfile")}
		select {
		case call := <-f.b.calls:
			t.Errorf("Expected build to not get called, but it did: %+v", call)
		case <-time.After(100 * time.Millisecond):
		}

		// fourth call: fix
		f.WriteFile("Tiltfile", `def foobar():
	start_fast_build("Dockerfile", "docker-tag")
	image = stop_build()
	return k8s_service("yaaaaaaaaml", image)
`)

		f.WriteFile("Dockerfile", `FROM iron/go:dev2`)
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Dockerfile")}
		call = <-f.b.calls
		assert.Equal(t, "FROM iron/go:dev2", call.manifest.BaseDockerfile)

		f.fsWatcher.errors <- endToken
	}()
	err = f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
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
	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)

	// This is currently a mostly uninteresting test because a fair amount of model/view isn't implemented
	expectedView := view.View{
		Resources: []view.Resource{
			{
				Name:                    "foobar",
				DirectoryWatched:        "/go",
				LatestFileChanges:       []string{},
				TimeSinceLastFileChange: 0,
				Status:                  view.ResourceStatusFresh,
				StatusDesc:              "",
			},
		},
	}

	assert.Equal(t, expectedView, f.hud.LastView)
	f.assertAllBuildsConsumed()
}

func testPod(podName string, manifestName string, phase string, creationTime time.Time) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              podName,
			CreationTimestamp: metav1.Time{Time: creationTime},
			Labels:            map[string]string{ManifestNameLabel: manifestName},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase(phase),
		},
	}
}

func testPodView(manifestName string, status view.ResourceStatus, statusDesc string) view.View {
	return view.View{
		Resources: []view.Resource{
			{
				Name:                    manifestName,
				DirectoryWatched:        "/go",
				LatestFileChanges:       []string{},
				TimeSinceLastFileChange: 0,
				Status:                  status,
				StatusDesc:              statusDesc,
			},
		},
	}
}

func TestPodEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates
		<-f.hud.Updates

		f.podEvents <- testPod("my pod", "foobar", "CrashLoopBackOff", time.Now())

		<-f.hud.Updates
		expectedView := testPodView("foobar", view.ResourceStatusBroken, "CrashLoopBackOff")
		assert.Equal(t, expectedView, f.hud.LastView)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
	f.assertAllHUDUpdatesConsumed()
}

func TestPodEventUpdateByTimestamp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	f.b.nextBuildFailure = errors.New("Build failed")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates
		<-f.hud.Updates

		firstCreationTime := time.Now()
		f.podEvents <- testPod("my pod", "foobar", "CrashLoopBackOff", firstCreationTime)

		<-f.hud.Updates
		f.podEvents <- testPod("my new pod", "foobar", "Running", firstCreationTime.Add(time.Minute*2))

		<-f.hud.Updates
		expectedView := testPodView("foobar", view.ResourceStatusFresh, "Running")
		assert.Equal(t, expectedView, f.hud.LastView)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
	f.assertAllHUDUpdatesConsumed()
}

func TestPodEventUpdateByPodName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	f.b.nextBuildFailure = errors.New("Build failed")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates
		<-f.hud.Updates

		creationTime := time.Now()
		f.podEvents <- testPod("my pod", "foobar", "CrashLoopBackOff", creationTime)

		<-f.hud.Updates
		f.podEvents <- testPod("my pod", "foobar", "Running", creationTime)

		<-f.hud.Updates
		expectedView := testPodView("foobar", view.ResourceStatusFresh, "Running")
		assert.Equal(t, expectedView, f.hud.LastView)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
	f.assertAllHUDUpdatesConsumed()
}

func TestPodEventIgnoreOlderPod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	endToken := errors.New("my-err-token")
	f.b.nextBuildFailure = errors.New("Build failed")
	go func() {
		call := <-f.b.calls
		assert.True(t, call.state.IsEmpty())
		<-f.hud.Updates
		<-f.hud.Updates

		creationTime := time.Now()
		f.podEvents <- testPod("my new pod", "foobar", "CrashLoopBackOff", creationTime)
		<-f.hud.Updates

		f.podEvents <- testPod("my pod", "foobar", "Running", creationTime.Add(time.Minute*-1))
		<-f.hud.Updates

		expectedView := testPodView("foobar", view.ResourceStatusBroken, "CrashLoopBackOff")
		assert.Equal(t, expectedView, f.hud.LastView)

		f.fsWatcher.errors <- endToken
	}()
	err := f.upper.CreateManifests(f.ctx, []model.Manifest{manifest}, true)
	assert.Equal(t, endToken, err)
	f.assertAllBuildsConsumed()
	f.assertAllHUDUpdatesConsumed()
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

func makeFakeFsWatcherMaker(fn *fakeNotify) fsWatcherMaker {
	return func() (watch.Notify, error) {
		return fn, nil
	}
}

func makeFakePodWatcherMaker(ch chan *v1.Pod) func(context.Context, *Store) error {
	return func(ctx context.Context, store *Store) error {
		go dispatchPodChangesLoop(ch, store)
		return nil
	}
}

type testFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	cancel     func()
	upper      Upper
	b          *fakeBuildAndDeployer
	fsWatcher  *fakeNotify
	timerMaker *fakeTimerMaker
	docker     *docker.FakeDockerClient
	hud        *hud.FakeHud
	podEvents  chan *v1.Pod
}

func newTestFixture(t *testing.T) *testFixture {
	f := tempdir.NewTempDirFixture(t)
	watcher := newFakeNotify()
	fsWatcherMaker := makeFakeFsWatcherMaker(watcher)
	b := newFakeBuildAndDeployer(t)

	podEvents := make(chan *v1.Pod)
	fakePodWatcherMaker := makeFakePodWatcherMaker(podEvents)

	timerMaker := makeFakeTimerMaker(t)
	docker := docker.NewFakeDockerClient()
	reaper := build.NewImageReaper(docker)

	k8s := k8s.NewFakeK8sClient()

	hud := hud.NewFakeHud()
	ctx, cancel := context.WithCancel(output.CtxForTest())

	upper := Upper{
		b:               b,
		fsWatcherMaker:  fsWatcherMaker,
		timerMaker:      timerMaker.maker(),
		podWatcherMaker: fakePodWatcherMaker,
		k8s:             k8s,
		browserMode:     BrowserAuto,
		reaper:          reaper,
		hud:             hud,
	}

	return &testFixture{
		f,
		ctx,
		cancel,
		upper,
		b,
		watcher,
		&timerMaker,
		docker,
		hud,
		podEvents,
	}
}

func (f *testFixture) TearDown() {
	f.TempDirFixture.TearDown()
	f.cancel()
}

func (f *testFixture) newManifest(name string, mounts []model.Mount) model.Manifest {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		f.T().Fatal(err)
	}

	return model.Manifest{Name: model.ManifestName(name), DockerRef: ref, Mounts: mounts}
}

func (f *testFixture) assertAllBuildsConsumed() {
	close(f.b.calls)

	for call := range f.b.calls {
		f.T().Fatalf("Build not consumed: %+v", call)
	}
}

func (f *testFixture) assertAllHUDUpdatesConsumed() {
	close(f.hud.Updates)

	for update := range f.hud.Updates {
		f.T().Fatalf("Update not consumed: %+v", update)
	}
}
