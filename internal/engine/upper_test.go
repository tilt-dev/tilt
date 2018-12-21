package engine

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/container"
	dockercompose "github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/logger"

	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/tiltfile2"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
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

var originalWD string

func init() {
	wd, _ := os.Getwd()
	originalWD = wd
}

const (
	simpleTiltfile = `
fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile')
k8s_resource('foobar', yaml='snack.yaml')
`
	simpleYAML    = testyaml.SnackYaml
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

	// Set this to simulate a container update that returns the container ID
	// it updated.
	nextBuildContainer container.ID

	// Set this to simulate the build failing. Do not set this directly, use fixture.SetNextBuildFailure
	nextBuildFailure error
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) nextBuildResult(ref reference.Named) store.BuildResult {
	b.buildCount++
	nt, _ := reference.WithTag(ref, fmt.Sprintf("tilt-%d", b.buildCount))
	containerID := b.nextBuildContainer
	b.nextBuildContainer = ""
	return store.BuildResult{
		Image:       nt,
		ContainerID: containerID,
	}
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

	// TODO(dmiller): change nextBuildResult to work with docker compose instead
	if manifest.IsDC() {
		return b.nextBuildResult(imageID), nil
	}
	return b.nextBuildResult(manifest.DockerInfo.DockerRef), nil
}

func (b *fakeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{
		t:     t,
		calls: make(chan buildAndDeployCall, 5),
	}
}

func TestUpper_Up(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar", nil)

	gYaml := model.NewYAMLManifest(model.ManifestName("my-global_yaml"),
		testyaml.BlorgBackendYAML, []string{"foo", "bar"})
	err := f.upper.Init(f.ctx, InitAction{
		Manifests:          []model.Manifest{manifest},
		GlobalYAMLManifest: gYaml,
	})
	close(f.b.calls)
	assert.Nil(t, err)
	var startedManifests []model.Manifest
	for call := range f.b.calls {
		startedManifests = append(startedManifests, call.manifest)
	}
	assert.Equal(t, []model.Manifest{manifest}, startedManifests)

	state := f.upper.store.RLockState()
	defer f.upper.store.RUnlockState()
	lines := strings.Split(string(state.ManifestStates[manifest.Name].LastBuild().Log), "\n")
	assert.Contains(t, lines, "fake building foobar")
	assert.Equal(t, gYaml, state.GlobalYAML)
}

func TestUpper_UpWatchError(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	f.fsWatcher.errors <- errors.New("bazquu")

	err := <-f.createManifestsResult
	if assert.NotNil(t, err) {
		assert.Equal(t, "bazquu", err.Error())
	}
}

func TestUpper_UpWatchFileChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	f.timerMaker.maxTimerLock.Lock()
	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)
	assert.Equal(t, []string{}, call.state.FilesChanged())

	fileRelPath := "fdas"
	f.fsWatcher.events <- watch.FileEvent{Path: fileRelPath}

	call = f.nextCall()
	assert.Equal(t, manifest, call.manifest)
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.state.LastImageAsString())
	fileAbsPath, err := filepath.Abs(fileRelPath)
	if err != nil {
		t.Errorf("error making abs path of %v: %v", fileRelPath, err)
	}
	assert.Equal(t, []string{fileAbsPath}, call.state.FilesChanged())

	f.WaitUntil("all builds complete", func(es store.EngineState) bool {
		return es.CurrentlyBuilding == ""
	})

	f.WithManifest("foobar", func(ms store.ManifestState) {
		assert.True(t, ms.LastBuild().Reason.Has(model.BuildReasonFlagMountFiles))
	})

	err = f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChanges(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	f.timerMaker.maxTimerLock.Lock()
	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)
	assert.Equal(t, []string{}, call.state.FilesChanged())

	f.timerMaker.restTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.events <- watch.FileEvent{Path: fileRelPath}
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.restTimerLock.Unlock()

	call = f.nextCall()
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

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChangesHitMaxTimeout(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)
	assert.Equal(t, []string{}, call.state.FilesChanged())

	f.timerMaker.maxTimerLock.Lock()
	f.timerMaker.restTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.events <- watch.FileEvent{Path: fileRelPath}
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.maxTimerLock.Unlock()

	call = f.nextCall()
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

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(errors.New("Build failed"))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	f.fsWatcher.events <- watch.FileEvent{Path: "/a.go"}

	call = f.nextCall()
	assert.True(t, call.state.IsEmpty())
	assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestFirstBuildCancelsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(context.Canceled)

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileNotWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	buildFailedToken := errors.New("doesn't compile")
	f.SetNextBuildFailure(buildFailedToken)

	err := f.upper.Init(f.ctx, InitAction{Manifests: []model.Manifest{manifest}})
	expectedErrStr := fmt.Sprintf("Build Failed: %v", buildFailedToken)
	assert.Equal(t, expectedErrStr, err.Error())
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall("first build")
	assert.True(t, call.state.IsEmpty())

	// Simulate a change to a.go that makes the build fail.
	f.SetNextBuildFailure(errors.New("build failed"))
	f.fsWatcher.events <- watch.FileEvent{Path: "/a.go"}

	call = f.nextCall("failed build from a.go change")
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.state.LastImageAsString())
	assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.FileEvent{Path: "/b.go"}

	// The next build should treat both a.go and b.go as changed, and build
	// on the last successful result, from before a.go changed.
	call = f.nextCall("build on last successful result")
	assert.Equal(t, []string{"/a.go", "/b.go"}, call.state.FilesChanged())
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.state.LastImageAsString())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestThreeBuilds(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall("first build")
	assert.True(t, call.state.IsEmpty())

	f.fsWatcher.events <- watch.FileEvent{Path: "/a.go"}

	call = f.nextCall("second build")
	assert.Equal(t, []string{"/a.go"}, call.state.FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.FileEvent{Path: "/b.go"}

	call = f.nextCall("third build")
	assert.Equal(t, []string{"/b.go"}, call.state.FilesChanged())

	f.waitForCompletedBuildCount(3)

	f.WithManifest("fe", func(ms store.ManifestState) {
		assert.Equal(t, 2, len(ms.BuildHistory))
		assert.Equal(t, []string{"/b.go"}, ms.BuildHistory[0].Edits)
		assert.Equal(t, []string{"/a.go"}, ms.BuildHistory[1].Edits)
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestRebuildWithSpuriousChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	// Simulate a change to .#a.go that's a broken symlink.
	realPath := filepath.Join(f.Path(), "a.go")
	tmpPath := filepath.Join(f.Path(), ".#a.go")
	_ = os.Symlink(realPath, tmpPath)

	f.fsWatcher.events <- watch.FileEvent{Path: tmpPath}

	f.assertNoCall()

	f.TouchFiles([]string{realPath})
	f.fsWatcher.events <- watch.FileEvent{Path: realPath}

	call = f.nextCall()
	assert.Equal(t, []string{realPath}, call.state.FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestRebuildDockerfileViaImageBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	// First call: with the old manifest
	call := f.nextCall("old manifest")
	assert.Empty(t, call.manifest.FastBuildInfo().BaseDockerfile)

	f.WriteConfigFiles("Dockerfile", `FROM iron/go:dev`)

	// Second call: new manifest!
	call = f.nextCall("new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, testyaml.SnackYAMLPostConfig, call.manifest.K8sInfo().YAML)

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.state.HasImage())

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("random_file.go")}

	// third call: new manifest should persist
	call = f.nextCall("persist new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.manifest.FastBuildInfo().BaseDockerfile)

	// Unchanged manifest --> we do NOT clear the build state
	assert.True(t, call.state.HasImage())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestMultipleChangesOnlyDeployOneManifest(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
fast_build("gcr.io/windmill-public-containers/servantes/snack", "Dockerfile1")
fast_build("gcr.io/windmill-public-containers/servantes/doggos", "Dockerfile2")

k8s_resource("baz", 'snack.yaml')
k8s_resource("quux", 'doggos.yaml')
`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("Dockerfile1", `FROM iron/go:dev1`)
	f.WriteFile("Dockerfile2", `FROM iron/go:dev2`)
	f.WriteFile("doggos.yaml", testyaml.DoggosDeploymentYaml)

	mount1 := model.Mount{LocalPath: f.JoinPath("mount1"), ContainerPath: "/go"}
	mount2 := model.Mount{LocalPath: f.JoinPath("mount2"), ContainerPath: "/go"}
	manifest1 := f.newManifest("baz", []model.Mount{mount1})
	manifest2 := f.newManifest("quux", []model.Mount{mount2})

	f.Start([]model.Manifest{manifest1, manifest2}, true)

	// First call: with the old manifests
	call := f.nextCall("old manifest (baz)")
	assert.Empty(t, call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "baz", string(call.manifest.Name))

	call = f.nextCall("old manifest (quux)")
	assert.Empty(t, call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "quux", string(call.manifest.Name))

	// rewrite the same config
	f.WriteConfigFiles("Dockerfile1", `FROM iron/go:dev1`)

	// Now with the manifests from the config files
	call = f.nextCall("manifest from config files (baz)")
	assert.Equal(t, `FROM iron/go:dev1`, call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "baz", string(call.manifest.Name))

	call = f.nextCall("manifest from config files (quux)")
	assert.Equal(t, `FROM iron/go:dev2`, call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "quux", string(call.manifest.Name))

	// Now change a dockerfile
	f.WriteConfigFiles("Dockerfile1", `FROM node:10`)

	// Second call: one new manifest!
	call = f.nextCall("changed config file --> new manifest")

	assert.Equal(t, "baz", string(call.manifest.Name))
	assert.ElementsMatch(t, []string{}, call.state.FilesChanged())

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.state.HasImage())

	// Importantly the other manifest, quux, is _not_ called
	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestSecondResourceIsBuilt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
fast_build("gcr.io/windmill-public-containers/servantes/snack", "Dockerfile1")

k8s_resource("baz", 'snack.yaml')
`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("Dockerfile1", `FROM iron/go:dev1`)
	f.WriteFile("Dockerfile2", `FROM iron/go:dev2`)
	f.WriteFile("doggos.yaml", testyaml.DoggosDeploymentYaml)

	manifests, _, _, err := tiltfile2.Load(f.ctx, tiltfile2.FileName, nil)
	assert.NoError(t, err)

	f.Start(manifests, true)

	// First call: with one resource
	call := f.nextCall("old manifest (baz)")
	assert.Equal(t, "FROM iron/go:dev1", call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "baz", string(call.manifest.Name))

	f.assertNoCall()

	// Now add a second resource
	f.WriteConfigFiles("Tiltfile", `
fast_build("gcr.io/windmill-public-containers/servantes/snack", "Dockerfile1")
fast_build("gcr.io/windmill-public-containers/servantes/doggos", "Dockerfile2")

k8s_resource("baz", 'snack.yaml')
k8s_resource("quux", 'doggos.yaml')
`)

	// Expect a build of quux, the new resource
	call = f.nextCall("changed config file --> new manifest")
	assert.Equal(t, "quux", string(call.manifest.Name))
	assert.ElementsMatch(t, []string{}, call.state.FilesChanged())

	err = f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestNoOpChangeToDockerfile(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
r = local_git_repo('.')
fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile') \
  .add(r, '.')
k8s_resource('foobar', 'snack.yaml')`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: with the old manifests
	call := f.nextCall("old manifests")
	assert.Equal(t, "FROM iron/go:dev1", call.manifest.FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "foobar", string(call.manifest.Name))

	f.WriteConfigFiles("Dockerfile", `FROM iron/go:dev1`)

	// The dockerfile hasn't changed, so there shouldn't be any builds.
	f.assertNoCall()

	f.store.Dispatch(manifestFilesChangedAction{
		files:        []string{f.JoinPath("random_file.go")},
		manifestName: "foobar",
	})

	// Second call: Editing the Dockerfile means we have to reevaluate the Tiltfile.
	// Editing the random file means we have to do a rebuild. BUT! The Dockerfile
	// hasn't changed, so the manifest hasn't changed, so we can do an incremental build.
	call = f.nextCall("incremental build despite edited config file")
	assert.Equal(t, "foobar", string(call.manifest.Name))
	assert.ElementsMatch(t, []string{
		f.JoinPath("random_file.go"),
	}, call.state.FilesChanged())

	// Unchanged manifest --> we do NOT clear the build state
	assert.True(t, call.state.HasImage())

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestRebuildDockerfileFailed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)

	// First call: with the old manifest
	call := f.nextCall("old manifest")
	assert.Empty(t, call.manifest.FastBuildInfo().BaseDockerfile)

	// second call: do some stuff
	f.WriteConfigFiles("Tiltfile", simpleTiltfile)

	call = f.nextCall("changed configs cause image build")
	assert.Equal(t, "FROM iron/go:dev", call.manifest.FastBuildInfo().BaseDockerfile)
	assert.False(t, call.state.HasImage()) // we cleared the previous build state to force an image build

	// Third call: error!
	f.WriteConfigFiles("Tiltfile", "borken")
	f.assertNoCall("Tiltfile error should prevent BuildAndDeploy from being called")

	// fourth call: fix
	f.WriteConfigFiles("Tiltfile", simpleTiltfile,
		"Dockerfile", `FROM iron/go:dev2`)

	call = f.nextCall("fixed broken config")
	assert.Equal(t, "FROM iron/go:dev2", call.manifest.FastBuildInfo().BaseDockerfile)
	assert.False(t, call.state.HasImage()) // we cleared the previous build state to force an image build
	f.WaitUntil("manifest definition order hasn't changed", func(state store.EngineState) bool {
		return len(state.ManifestDefinitionOrder) == 1
	})
	f.WaitUntilManifest("LastError was cleared", "foobar", func(state store.ManifestState) bool {
		return state.LastBuild().Error == nil
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestBreakManifest(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	origTiltfile := `
fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile') \
	.add(local_git_repo('./nested'), '.')  # Tiltfile is not mounted
k8s_resource('foobar', yaml='snack.yaml')`

	f.MkdirAll("nested/.git") // Spoof a git directory -- this is what we'll mount.
	f.WriteFile("Tiltfile", origTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: all is well
	_ = f.nextCall("first call")

	// Second call: change Tiltfile, break manifest
	f.WriteConfigFiles("Tiltfile", "borken")
	f.assertNoCall("Tiltfile error should prevent BuildAndDeploy from being called")

	f.WaitUntil("error set", func(st store.EngineState) bool {
		return st.LastTiltfileError != nil
	})

	f.withState(func(es store.EngineState) {
		assert.Equal(t, "", nextManifestToBuild(es).String())
	})
}

func TestBreakAndUnbreakManifestWithNoChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	origTiltfile := `
fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile') \
	.add(local_git_repo('./nested'), '.')  # Tiltfile is not mounted
k8s_resource('foobar', yaml='snack.yaml')`
	f.MkdirAll("nested/.git") // Spoof a git directory -- this is what we'll mount.
	f.WriteFile("Tiltfile", origTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: all is well
	_ = f.nextCall("first call")

	// Second call: change Tiltfile, break manifest
	f.WriteConfigFiles("Tiltfile", "borken")
	f.WaitUntil("state is broken", func(st store.EngineState) bool {
		return st.LastTiltfileError != nil
	})

	// Third call: put Tiltfile back. No change to manifest or to mounted files, so expect no build.
	f.WriteConfigFiles("Tiltfile", origTiltfile)
	f.WaitUntil("state is restored", func(st store.EngineState) bool {
		return st.LastTiltfileError == nil
	})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, "", nextManifestToBuild(state).String())
	})
}

func TestBreakAndUnbreakManifestWithChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	tiltfileString := func(cmd string) string {
		return fmt.Sprintf(`fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile') \
	.add(local_git_repo('./nested'), '.') \
  .run('%s')
k8s_resource('foobar', 'snack.yaml')

`, cmd)
	}

	f.MkdirAll("nested/.git") // Spoof a git directory -- this is what we'll mount.
	f.WriteFile("Tiltfile", tiltfileString("original"))
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	f.WaitUntil("first build finished", func(state store.EngineState) bool {
		return state.CompletedBuildCount == 1
	})

	name := "foobar"
	// Second call: change Tiltfile, break manifest
	f.WriteConfigFiles("Tiltfile", "borken")
	f.WaitUntil("manifest load error", func(st store.EngineState) bool {
		return st.LastTiltfileError != nil
	})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, 1, state.CompletedBuildCount)
	})

	// Third call: put Tiltfile back. manifest changed, so expect a build
	f.WriteConfigFiles("Tiltfile", tiltfileString("changed"))

	f.WaitUntil("second build finished", func(state store.EngineState) bool {
		return state.CompletedBuildCount == 2
	})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, "", nextManifestToBuild(state).String())
		assert.NoError(t, state.LastTiltfileError)
	})

	f.withManifestState(name, func(ms store.ManifestState) {
		expectedSteps := []model.Step{{
			Cmd:           model.ToShellCmd("changed"),
			BaseDirectory: f.Path(),
		}}
		assert.Equal(t, expectedSteps, ms.Manifest.FastBuildInfo().Steps)
	})
}

func TestStaticRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	df := `FROM golang
ADD ./ ./
go build ./...
`
	manifest := f.newManifest("foobar", nil)
	manifest.DockerInfo = manifest.DockerInfo.WithBuildDetails(
		model.StaticBuild{
			Dockerfile: df,
			BuildPath:  f.Path(),
		})

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall("first build")
	assert.True(t, call.state.IsEmpty())

	// Simulate a change to main.go
	mainPath := filepath.Join(f.Path(), "main.go")
	f.fsWatcher.events <- watch.FileEvent{Path: mainPath}

	// Check that this triggered a rebuild.
	call = f.nextCall("rebuild triggered")
	assert.Equal(t, []string{mainPath}, call.state.FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

// Checks that the image reaper kicks in and GCs old images.
func TestReapOldBuilds(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.docker.BuildCount++

	f.Start([]model.Manifest{manifest}, true)

	f.PollUntil("images reaped", func() bool {
		return len(f.docker.RemovedImageIDs) > 0
	})

	assert.Equal(t, []string{"build-id-0"}, f.docker.RemovedImageIDs)
	err := f.Stop()
	assert.Nil(t, err)
}

func TestHudUpdated(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	oldPWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldPWD)
	os.Chdir(f.TempDirFixture.Path())

	mount := model.Mount{LocalPath: f.TempDirFixture.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0
	})

	err = f.Stop()
	assert.Equal(t, nil, err)

	assert.Equal(t, 1, len(f.hud.LastView.Resources))
	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, manifest.Name, model.ManifestName(rv.Name))
	assert.Equal(t, ".", rv.DirectoriesWatched[0])
	f.assertAllBuildsConsumed()
}

func (f *testFixture) testPod(podID string, manifestName string, phase string, cID string, creationTime time.Time) *v1.Pod {
	msgs := validation.NameIsDNSSubdomain(podID, false)
	if len(msgs) != 0 {
		f.T().Fatalf("pod id %q is invalid: %s", podID, msgs)
	}

	var containerID string
	if cID != "" {
		containerID = fmt.Sprintf("%s%s", k8s.ContainerIDPrefix, cID)
	}

	// Use the pod ID as the image tag. This is kind of weird, but gets at the semantics
	// we want (e.g., a new pod ID indicates that this is a new build).
	// Tests that don't want this behavior should replace the image with setImage(pod, imageName)
	image := fmt.Sprintf("%s:%s", f.imageNameForManifest(manifestName).String(), podID)
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              podID,
			CreationTimestamp: metav1.Time{Time: creationTime},
			Labels:            map[string]string{ManifestNameLabel: manifestName},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "main",
					Image: image,
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase(phase),
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:        "test container!",
					Image:       image,
					Ready:       true,
					ContainerID: containerID,
				},
			},
		},
	}
}

func setImage(pod *v1.Pod, image string) {
	pod.Spec.Containers[0].Image = image
	pod.Status.ContainerStatuses[0].Image = image
}

func setRestartCount(pod *v1.Pod, restartCount int) {
	pod.Status.ContainerStatuses[0].RestartCount = int32(restartCount)
}

func TestPodEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	f.podEvent(f.testPod("my-pod", "foobar", "CrashLoopBackOff", testContainer, time.Now()))

	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].PodName == "my-pod"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-pod", rv.PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	past := time.Now().Add(-time.Minute)
	now := time.Now()
	imagePast := fmt.Sprintf("%s:%s", f.imageNameForManifest("fe").String(), "past")
	imageNow := fmt.Sprintf("%s:%s", f.imageNameForManifest("fe").String(), "now")
	podAPast := f.testPod("pod-a", "fe", "Running", testContainer, past)
	podBPast := f.testPod("pod-b", "fe", "Running", testContainer, past)
	podANow := f.testPod("pod-a", "fe", "Running", testContainer, now)
	podBNow := f.testPod("pod-b", "fe", "Running", testContainer, now)
	podCNow := f.testPod("pod-b", "fe", "Running", testContainer, now)
	podCNowDeleting := f.testPod("pod-c", "fe", "Running", testContainer, now)
	podCNowDeleting.DeletionTimestamp = &metav1.Time{Time: now}

	setImage(podAPast, imagePast)
	setImage(podBPast, imagePast)
	setImage(podANow, imageNow)
	setImage(podBNow, imageNow)
	setImage(podCNow, imageNow)
	setImage(podCNowDeleting, imageNow)

	// Test the pod events coming in in different orders,
	// and the manifest ends up with podANow and podBNow
	podOrders := [][]*v1.Pod{
		{podAPast, podBPast, podANow, podBNow},
		{podAPast, podANow, podBPast, podBNow},
		{podANow, podAPast, podBNow, podBPast},
		{podAPast, podBPast, podANow, podCNow, podCNowDeleting, podBNow},
	}

	for i, order := range podOrders {

		t.Run(fmt.Sprintf("TestPodOrder%d", i), func(t *testing.T) {
			f := newTestFixture(t)
			defer f.TearDown()
			mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
			manifest := f.newManifest("fe", []model.Mount{mount})
			f.Start([]model.Manifest{manifest}, true)

			call := f.nextCall()
			assert.True(t, call.state.IsEmpty())

			for _, pod := range order {
				f.podEvent(pod)
			}

			f.upper.store.Dispatch(PodLogAction{
				ManifestName: "fe",
				PodID:        k8s.PodIDFromPod(podBNow),
				Log:          []byte("pod b log\n"),
			})

			f.WaitUntilManifest("pod log seen", "fe", func(ms store.ManifestState) bool {
				return strings.Contains(ms.MostRecentPod().Log(), "pod b log")
			})

			f.WithManifest("fe", func(ms store.ManifestState) {
				assert.Equal(t, 2, ms.PodSet.Len())
				assert.Equal(t, now, ms.PodSet.Pods["pod-a"].StartedAt)
				assert.Equal(t, now, ms.PodSet.Pods["pod-b"].StartedAt)
				assert.Equal(t, imageNow, ms.PodSet.ImageID.String())
			})

			assert.NoError(t, f.Stop())
		})
	}
}

func TestPodEventContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	var ref reference.NamedTagged
	f.WaitUntilManifest("image appears", "foobar", func(ms store.ManifestState) bool {
		ref = ms.LastSuccessfulResult.Image
		return ref != nil
	})

	pod := f.testPod("my-pod", "foobar", "Running", testContainer, time.Now())
	pod.Status = k8s.FakePodStatus(ref, "Running")
	pod.Status.ContainerStatuses[0].ContainerID = ""
	pod.Spec = k8s.FakePodSpec(ref)

	f.podEvent(pod)

	podState := store.Pod{}
	f.WaitUntilManifest("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.MostRecentPod()
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
	f.WaitUntil("waiting for builds to be ready", func(st store.EngineState) bool {
		return nextManifestToBuild(st) == manifest.Name
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

	f.WaitUntilManifest("NeedsRebuildFromCrash set to True", "foobar", func(state store.ManifestState) bool {
		return state.NeedsRebuildFromCrash
	})
	// wait for triggered image build (count is 1 because our fake build above doesn't increment this number).
	f.waitForCompletedBuildCount(1)
}

func TestPodUnexpectedContainerStartsImageBuildOutOfOrderEvents(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)

	// Start a fake build to set manifestState.ExpectedContainerId
	f.store.Dispatch(manifestFilesChangedAction{
		manifestName: manifest.Name,
		files:        []string{"/go/a"},
	})
	f.WaitUntil("waiting for builds to be ready", func(st store.EngineState) bool {
		return nextManifestToBuild(st) == manifest.Name
	})
	f.store.Dispatch(BuildStartedAction{
		Manifest:  manifest,
		StartTime: time.Now(),
	})

	// Simulate k8s restarting the container due to a crash.
	f.podEvent(f.testPod("mypod", "foobar", "Running", "myfunnycontainerid", time.Now()))
	f.store.Dispatch(BuildCompleteAction{
		Result: store.BuildResult{
			Image:       nil,
			ContainerID: "theOriginalContainer",
		},
	})

	f.WaitUntilManifest("NeedsRebuildFromCrash set to True", "foobar", func(state store.ManifestState) bool {
		return state.NeedsRebuildFromCrash
	})
	// wait for triggered image build (count is 1 because our fake build above doesn't increment this number).
	f.waitForCompletedBuildCount(1)
}

func TestPodUnexpectedContainerAfterInPlaceUpdate(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)

	// Start a fake build to set manifestState.ExpectedContainerId
	f.store.Dispatch(manifestFilesChangedAction{
		manifestName: manifest.Name,
		files:        []string{"/go/a"},
	})
	f.WaitUntil("waiting for builds to be ready", func(st store.EngineState) bool {
		return nextManifestToBuild(st) == manifest.Name
	})

	f.store.Dispatch(BuildStartedAction{
		Manifest:  manifest,
		StartTime: time.Now(),
	})

	// Simulate a normal build completion
	podStartTime := time.Now()
	f.store.Dispatch(BuildCompleteAction{
		Result: store.BuildResult{
			Image:       nil,
			ContainerID: "normal-container-id",
		},
	})
	f.podEvent(f.testPod("mypod", "foobar", "Running", "normal-container-id", podStartTime))

	// Start another fake build to set manifestState.ExpectedContainerId
	f.store.Dispatch(manifestFilesChangedAction{
		manifestName: manifest.Name,
		files:        []string{"/go/a"},
	})
	f.WaitUntil("waiting for builds to be ready", func(st store.EngineState) bool {
		return nextManifestToBuild(st) == manifest.Name
	})
	f.store.Dispatch(BuildStartedAction{
		Manifest:  manifest,
		StartTime: time.Now(),
	})

	// Simulate a pod crash, then a build compltion
	f.podEvent(f.testPod("mypod", "foobar", "Running", "funny-container-id", podStartTime))
	f.store.Dispatch(BuildCompleteAction{
		Result: store.BuildResult{
			Image:       nil,
			ContainerID: "normal-container-id",
		},
	})

	f.WaitUntilManifest("NeedsRebuildFromCrash set to True", "foobar", func(state store.ManifestState) bool {
		return state.NeedsRebuildFromCrash
	})
}

func TestPodEventUpdateByTimestamp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(errors.New("Build failed"))
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	firstCreationTime := time.Now()
	f.podEvent(f.testPod("my-pod", "foobar", "CrashLoopBackOff", testContainer, firstCreationTime))
	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-new-pod", "foobar", "Running", testContainer, firstCreationTime.Add(time.Minute*2)))
	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].PodStatus == "Running"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-new-pod", rv.PodName)
	assert.Equal(t, "Running", rv.PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventUpdateByPodName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(errors.New("Build failed"))
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	f.waitForCompletedBuildCount(1)

	creationTime := time.Now()
	f.podEvent(f.testPod("my-pod", "foobar", "CrashLoopBackOff", testContainer, creationTime))

	f.WaitUntilHUD("pod crashes", func(view view.View) bool {
		rv := view.Resources[0]
		return rv.PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-pod", "foobar", "Running", testContainer, creationTime))

	f.WaitUntilHUD("pod comes back", func(view view.View) bool {
		rv := view.Resources[0]
		return rv.PodStatus == "Running"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-pod", rv.PodName)
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
	f.SetNextBuildFailure(errors.New("Build failed"))
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.state.IsEmpty())

	creationTime := time.Now()
	f.podEvent(f.testPod("my-new-pod", "foobar", "CrashLoopBackOff", testContainer, creationTime))
	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-pod", "foobar", "Running", testContainer, creationTime.Add(time.Minute*-1)))
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-new-pod", rv.PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.PodStatus)
}

func TestPodContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	_ = f.nextCall()

	var ref reference.NamedTagged
	f.WaitUntilManifest("image appears", "fe", func(ms store.ManifestState) bool {
		ref = ms.LastSuccessfulResult.Image
		return ref != nil
	})

	startedAt := time.Now()
	f.podEvent(f.testPod("pod-id", "fe", "Running", testContainer, startedAt))
	f.WaitUntilManifest("pod appears", "fe", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == "pod-id"
	})

	pod := f.testPod("pod-id", "fe", "Running", testContainer, startedAt)
	pod.Spec = k8s.FakePodSpec(ref)
	pod.Status = k8s.FakePodStatus(ref, "Running")
	f.podEvent(pod)

	f.WaitUntilManifest("container is ready", "fe", func(ms store.ManifestState) bool {
		ports := ms.MostRecentPod().ContainerPorts
		return len(ports) == 1 && ports[0] == 8080
	})

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestUpper_WatchDockerIgnoredFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount}).
		WithDockerignores([]model.Dockerignore{
			{
				LocalPath: f.Path(),
				Contents:  "dignore.txt",
			},
		})

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("dignore.txt")}
	f.assertNoCall("event for ignored file should not trigger build")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpper_WatchGitIgnoredFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount}).
		WithRepos([]model.LocalGithubRepo{
			{
				LocalPath:         f.Path(),
				GitignoreContents: "gignore.txt",
			},
		})

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("gignore.txt")}
	f.assertNoCall("event for ignored file should not trigger build")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
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

	f.WithManifest(name, func(ms store.ManifestState) {
		assert.Equal(t, "second string\n", ms.MostRecentPod().Log())
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestBuildResetsPodLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(name)
	f.podLog(name, "first string")

	f.WithManifest(name, func(ms store.ManifestState) {
		assert.Equal(t, "first string\n", ms.MostRecentPod().Log())
	})

	f.upper.store.Dispatch(manifestFilesChangedAction{
		manifestName: "foobar",
		files:        []string{"/go/a.go"},
	})

	f.waitForCompletedBuildCount(2)

	f.WithManifest(name, func(ms store.ManifestState) {
		assert.Equal(t, "", ms.MostRecentPod().Log())
		assert.Equal(t, ms.LastBuild().StartTime, ms.MostRecentPod().UpdateStartTime)
	})
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

	// the third instance is still up, so we want to show the log from the last crashed pod plus the log from the current pod
	f.WithManifest(name, func(ms store.ManifestState) {
		assert.Equal(t, "second string\nthird string\n", ms.MostRecentPod().Log())
	})

	err := f.Stop()
	assert.NoError(t, err)
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

	// The second instance is down, so we don't include the first instance's log
	f.WithManifest(name, func(ms store.ManifestState) {
		assert.Equal(t, "second string\n", ms.MostRecentPod().Log())
	})

	err := f.Stop()
	assert.NoError(t, err)
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

	svc := testService("myservice", "foobar", "1.2.3.4", 8080)
	dispatchServiceChange(f.store, svc, "")

	f.WaitUntilManifest("lb updated", "foobar", func(ms store.ManifestState) bool {
		return len(ms.LBs) > 0
	})

	err := f.Stop()
	assert.NoError(t, err)

	ms := f.upper.store.RLockState().ManifestStates[manifest.Name]
	defer f.upper.store.RUnlockState()
	assert.Equal(t, 1, len(ms.LBs))
	url, ok := ms.LBs["myservice"]
	if !ok {
		t.Fatalf("%v did not contain key 'myservice'", ms.LBs)
	}
	assert.Equal(t, "http://1.2.3.4:8080/", url.String())
}

func TestUpper_ServiceEventRemovesURL(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	svc := testService("myservice", "foobar", "1.2.3.4", 8080)
	dispatchServiceChange(f.store, svc, "")

	f.WaitUntilManifest("lb url added", "foobar", func(ms store.ManifestState) bool {
		url := ms.LBs["myservice"]
		if url == nil {
			return false
		}
		return "http://1.2.3.4:8080/" == url.String()
	})

	svc = testService("myservice", "foobar", "1.2.3.4", 8080)
	svc.Status = v1.ServiceStatus{}
	dispatchServiceChange(f.store, svc, "")

	f.WaitUntilManifest("lb url removed", "foobar", func(ms store.ManifestState) bool {
		url := ms.LBs["myservice"]
		return url == nil
	})

	err := f.Stop()
	assert.NoError(t, err)
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
	assert.NoError(t, err)
}

func TestInitWithGlobalYAML(t *testing.T) {
	f := newTestFixture(t)
	state := f.store.RLockState()
	ym := model.NewYAMLManifest(model.ManifestName("global"), testyaml.BlorgBackendYAML, []string{})
	state.GlobalYAML = ym
	f.store.RUnlockState()
	f.Start([]model.Manifest{}, true)
	f.store.Dispatch(InitAction{
		Manifests:          []model.Manifest{},
		GlobalYAMLManifest: ym,
	})
	f.WaitUntil("global YAML manifest gets set on init", func(st store.EngineState) bool {
		return st.GlobalYAML.K8sYAML() == testyaml.BlorgBackendYAML
	})

	newYM := model.NewYAMLManifest(model.ManifestName("global"), testyaml.BlorgJobYAML, []string{})
	f.store.Dispatch(ConfigsReloadedAction{
		GlobalYAML: newYM,
	})

	f.WaitUntil("global YAML manifest gets updated", func(st store.EngineState) bool {
		return st.GlobalYAML.K8sYAML() == testyaml.BlorgJobYAML
	})
}

func TestInitSetsTiltfilePath(t *testing.T) {
	f := newTestFixture(t)
	f.Start([]model.Manifest{}, true)
	f.store.Dispatch(InitAction{
		Manifests:    []model.Manifest{},
		TiltfilePath: "/Tiltfile",
	})
	f.WaitUntil("tiltfile path gets set on init", func(st store.EngineState) bool {
		return st.TiltfilePath == "/Tiltfile"
	})
}

func TestHudExitNoError(t *testing.T) {
	f := newTestFixture(t)
	f.Start([]model.Manifest{}, true)
	f.store.Dispatch(hud.NewExitAction(nil))
	err := f.WaitForExit()
	assert.NoError(t, err)
}

func TestHudExitWithError(t *testing.T) {
	f := newTestFixture(t)
	f.Start([]model.Manifest{}, true)
	e := errors.New("helllllo")
	f.store.Dispatch(hud.NewExitAction(e))
	err := f.WaitForExit()
	assert.Equal(t, e, err)
}

func TestNewMountsAreWatched(t *testing.T) {
	f := newTestFixture(t)
	mount1 := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	m1 := f.newManifest("mani1", []model.Mount{mount1})
	f.Start([]model.Manifest{
		m1,
	}, true)

	f.waitForCompletedBuildCount(1)

	mount2 := model.Mount{LocalPath: "/js", ContainerPath: "/go"}
	m2 := f.newManifest("mani1", []model.Mount{mount1, mount2})
	f.store.Dispatch(ConfigsReloadedAction{
		Manifests: []model.Manifest{m2},
	})

	f.WaitUntilManifest("has new mounts", "mani1", func(st store.ManifestState) bool {
		return len(st.Manifest.FastBuildInfo().Mounts) == 2
	})

	f.PollUntil("watches setup", func() bool {
		watches, ok := f.fwm.manifestWatches[m2.ManifestName()]
		if !ok {
			return false
		}
		return len(watches.manifest.Dependencies()) == 2
	})
}

func TestDockerComposeRecordsLogs(t *testing.T) {
	f := newTestFixture(t)
	m := f.setupHAProxyDCFixture()
	expected := "spoonerisms_1  | 2018-12-20T16:11:04.070480042Z yarn install v1.10."
	f.dcc.SetLogOutput(expected + "\n")

	f.Start([]model.Manifest{
		m,
	}, true)

	f.waitForCompletedBuildCount(1)
	// recorded in global log
	assert.Contains(t, f.LogLines(), expected)

	// recorded on manifest state
	f.withManifestState(m.ManifestName().String(), func(st store.ManifestState) {
		assert.Contains(t, st.DCResourceState().Log(), expected)
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

type testFixture struct {
	*tempdir.TempDirFixture
	ctx                   context.Context
	cancel                func()
	upper                 Upper
	b                     *fakeBuildAndDeployer
	fsWatcher             *fakeMultiWatcher
	timerMaker            *fakeTimerMaker
	docker                *docker.FakeDockerClient
	hud                   *hud.FakeHud
	createManifestsResult chan error
	log                   *bufsync.ThreadSafeBuffer
	store                 *store.Store
	pod                   *v1.Pod
	bc                    *BuildController
	fwm                   *WatchManager
	cc                    *ConfigsController
	originalWD            string
	dcc                   *dockercompose.FakeDCClient

	onchangeCh chan bool
}

func newTestFixture(t *testing.T) *testFixture {
	f := tempdir.NewTempDirFixture(t)
	watcher := newFakeMultiWatcher()
	b := newFakeBuildAndDeployer(t)

	timerMaker := makeFakeTimerMaker(t)

	docker := docker.NewFakeDockerClient()
	reaper := build.NewImageReaper(docker)

	k8s := k8s.NewFakeK8sClient()
	pw := NewPodWatcher(k8s)
	sw := NewServiceWatcher(k8s, "")

	fakeHud := hud.NewFakeHud()

	log := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(testoutput.ForkedCtxForTest(log))

	fSub := fixtureSub{ch: make(chan bool, 1000)}
	st := store.NewStore(UpperReducer, store.LogActionsFlag(false))
	st.AddSubscriber(fSub)

	plm := NewPodLogManager(k8s)
	bc := NewBuildController(b)

	_ = os.Chdir(f.Path())
	_ = os.Mkdir(f.JoinPath(".git"), os.FileMode(0777))

	fwm := NewWatchManager(watcher.newSub, timerMaker.maker())
	pfc := NewPortForwardController(k8s)
	ic := NewImageController(reaper)
	gybc := NewGlobalYAMLBuildController(k8s)
	cc := NewConfigsController()
	dcc := &dockercompose.FakeDCClient{}
	dcw := NewDockerComposeEventWatcher(dcc)
	dclm := NewDockerComposeLogManager(dcc)
	upper := NewUpper(ctx, fakeHud, pw, sw, st, plm, pfc, fwm, bc, ic, gybc, cc, k8s, dcw, dclm)

	go func() {
		fakeHud.Run(ctx, upper.Dispatch, hud.DefaultRefreshInterval)
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
		hud:            fakeHud,
		log:            log,
		store:          st,
		bc:             bc,
		onchangeCh:     fSub.ch,
		fwm:            fwm,
		cc:             cc,
		originalWD:     originalWD,
		dcc:            dcc,
	}
}

func (f *testFixture) Start(manifests []model.Manifest, watchMounts bool) {
	f.Init(InitAction{
		Manifests:   manifests,
		WatchMounts: watchMounts,
	})
}

func (f *testFixture) Init(action InitAction) {
	if action.TiltfilePath == "" {
		action.TiltfilePath = "/Tiltfile"
	}

	manifests := action.Manifests
	watchMounts := action.WatchMounts
	f.createManifestsResult = make(chan error)

	go func() {
		err := f.upper.Init(f.ctx, action)
		if err != nil && err != context.Canceled {
			// Print this out here in case the test never completes
			log.Printf("CreateManifests failed: %v", err)
			f.cancel()
		}
		f.createManifestsResult <- err
	}()

	f.WaitUntil("manifests appear", func(st store.EngineState) bool {
		return len(st.ManifestStates) == len(manifests) && st.WatchMounts == watchMounts
	})

	f.PollUntil("watches setup", func() bool {
		return !watchMounts || len(f.fwm.manifestWatches) == len(manifests)
	})
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

func (f *testFixture) WaitForExit() error {
	select {
	case <-time.After(time.Second):
		f.T().Fatalf("Timed out waiting for upper to exit")
		return nil
	case err := <-f.createManifestsResult:
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

// Wait until the given view test passes.
func (f *testFixture) WaitUntilHUD(msg string, isDone func(view.View) bool) {
	f.hud.WaitUntil(f.T(), f.ctx, msg, isDone)
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
			// dump the stacks of all goroutines
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)

			fmt.Printf("state: '%+v'\n", state)

			f.T().Fatalf("Timed out waiting for: %s", msg)
		case <-f.onchangeCh:
		}
	}
}

func (f *testFixture) withState(tf func(store.EngineState)) {
	state := f.upper.store.RLockState()
	defer f.upper.store.RUnlockState()
	tf(state)
}

func (f *testFixture) withManifestState(name string, tf func(ms store.ManifestState)) {
	f.withState(func(es store.EngineState) {
		ms, ok := es.ManifestStates[model.ManifestName(name)]
		if !ok {
			f.T().Fatalf("no manifest state for name %s", name)
		}
		tf(*ms)
	})
}

// Poll until the given state passes. This should be used for checking things outside
// the state loop. Don't use this to check state inside the state loop.
func (f *testFixture) PollUntil(msg string, isDone func() bool) {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	for {
		done := isDone()
		if done {
			return
		}

		select {
		case <-ctx.Done():
			f.T().Fatalf("Timed out waiting for: %s", msg)
		case <-ticker.C:
		}
	}
}

func (f *testFixture) WithManifest(name model.ManifestName, test func(store.ManifestState)) {
	state := f.upper.store.RLockState()
	defer f.upper.store.RUnlockState()

	ms := state.ManifestStates[name]
	if ms == nil {
		f.T().Fatalf("Missing manifest: %s", name)
	}
	test(*ms)
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

func (f *testFixture) nextCall(msgAndArgs ...interface{}) buildAndDeployCall {
	msg := "timed out waiting for BuildAndDeployCall"
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprintf("timed out waiting for BuildAndDeployCall: %s", msgAndArgs...)
	}

	for {
		select {
		case call := <-f.b.calls:
			return call
		case <-time.After(100 * time.Millisecond):
			f.T().Fatal(msg)
		}
	}
}

func (f *testFixture) assertNoCall(msgAndArgs ...interface{}) {
	msg := "expected there to be no BuildAndDeployCalls, but found one"
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprintf("expected there to be no BuildAndDeployCalls, but found one: %s", msgAndArgs...)
	}
	for {
		select {
		case <-f.b.calls:
			f.T().Fatal(msg)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func (f *testFixture) startPod(manifestName model.ManifestName) {
	pID := k8s.PodID("mypod")
	f.pod = f.testPod(pID.String(), manifestName.String(), "Running", testContainer, time.Now())
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifest("pod appears", manifestName.String(), func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == k8s.PodID(f.pod.Name)
	})
}

func (f *testFixture) podLog(manifestName model.ManifestName, s string) {
	f.upper.store.Dispatch(PodLogAction{
		ManifestName: manifestName,
		PodID:        k8s.PodID(f.pod.Name),
		Log:          []byte(s + "\n"),
	})

	f.WaitUntilManifest("pod log seen", string(manifestName), func(ms store.ManifestState) bool {
		return strings.Contains(string(ms.MostRecentPod().CurrentLog), s)
	})
}

func (f *testFixture) restartPod() {
	restartCount := f.pod.Status.ContainerStatuses[0].RestartCount + 1
	f.pod.Status.ContainerStatuses[0].RestartCount = restartCount
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifest("pod restart seen", "foobar", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().ContainerRestarts == int(restartCount)
	})
}

func (f *testFixture) notifyAndWaitForPodStatus(pred func(pod store.Pod) bool) {
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifest("pod status change seen", "foobar", func(state store.ManifestState) bool {
		return pred(state.MostRecentPod())
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
	close(f.fsWatcher.events)
	close(f.fsWatcher.errors)
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
	return model.Manifest{
		Name: model.ManifestName(name),
		DockerInfo: model.DockerInfo{DockerRef: ref}.
			WithBuildDetails(model.FastBuild{Mounts: mounts}),
	}
}

func (f *testFixture) newDCManifest(name string, DCYAMLRaw string, dockerfileContents string) model.Manifest {
	f.WriteFile("docker-compose.yml", DCYAMLRaw)
	return model.Manifest{
		Name: model.ManifestName(name),
	}.WithDeployInfo(model.DCInfo{
		ConfigPath: f.JoinPath("docker-compose.yml"),
		YAMLRaw:    []byte(DCYAMLRaw),
		DfRaw:      []byte(dockerfileContents),
	})
}

func (f *testFixture) assertAllBuildsConsumed() {
	close(f.b.calls)

	for call := range f.b.calls {
		f.T().Fatalf("Build not consumed: %+v", call)
	}
}

func (f *testFixture) loadAndStart() {
	manifests, _, _, err := tiltfile2.Load(f.ctx, f.JoinPath(tiltfile2.FileName), nil)
	if err != nil {
		f.T().Fatal(err)
	}
	f.Start(manifests, true)
}

func (f *testFixture) WriteConfigFiles(args ...string) {
	if (len(args) % 2) != 0 {
		f.T().Fatalf("WriteConfigFiles needs an even number of arguments; got %d", len(args))
	}

	var filenames []string
	for i := 0; i < len(args); i += 2 {
		f.WriteFile(args[i], args[i+1])
		filenames = append(filenames, args[i])
	}
	f.store.Dispatch(manifestFilesChangedAction{manifestName: ConfigsManifestName, files: filenames})
}

func (f *testFixture) setupHAProxyDCFixture() model.Manifest {
	dcp := filepath.Join(f.originalWD, "testdata", "haproxy_docker-config.yml")
	dcpc, err := ioutil.ReadFile(dcp)
	if err != nil {
		f.T().Fatal(err)
	}
	f.TempDirFixture.WriteFile("docker-config.yml", string(dcpc))

	dfp := filepath.Join(f.originalWD, "testdata", "haproxy.dockerfile")
	dfc, err := ioutil.ReadFile(dfp)
	if err != nil {
		f.T().Fatal(err)
	}
	f.TempDirFixture.WriteFile("Dockerfile", string(dfc))

	hcp := filepath.Join(f.originalWD, "testdata", "haproxy.cfg")
	hcc, err := ioutil.ReadFile(hcp)
	if err != nil {
		f.T().Fatal(err)
	}
	f.TempDirFixture.WriteFile("haproxy.cfg", string(hcc))

	return f.newDCManifest("haproxy", string(dcpc), string(dfc))
}

type fixtureSub struct {
	ch chan bool
}

func (s fixtureSub) OnChange(ctx context.Context, st store.RStore) {
	s.ch <- true
}
