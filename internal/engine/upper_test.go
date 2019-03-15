package engine

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	testoutput "github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/watch"
)

var originalWD string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	originalWD = wd
}

const (
	simpleTiltfile = `
repo = local_git_repo('.')
img = fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile')
img.add(repo, '/src')
k8s_resource('foobar', yaml='snack.yaml')
`
	simpleYAML    = testyaml.SnackYaml
	testContainer = "myTestContainer"
)

const testDeployID = model.DeployID(1234567890)

// represents a single call to `BuildAndDeploy`
type buildAndDeployCall struct {
	count int
	specs []model.TargetSpec
	state store.BuildStateSet
}

func (c buildAndDeployCall) image() model.ImageTarget {
	for _, spec := range c.specs {
		t, ok := spec.(model.ImageTarget)
		if ok {
			return t
		}
	}
	return model.ImageTarget{}
}

func (c buildAndDeployCall) k8s() model.K8sTarget {
	for _, spec := range c.specs {
		t, ok := spec.(model.K8sTarget)
		if ok {
			return t
		}
	}
	return model.K8sTarget{}
}

func (c buildAndDeployCall) dc() model.DockerComposeTarget {
	for _, spec := range c.specs {
		t, ok := spec.(model.DockerComposeTarget)
		if ok {
			return t
		}
	}
	return model.DockerComposeTarget{}
}

func (c buildAndDeployCall) oneState() store.BuildState {
	if len(c.state) != 1 {
		panic(fmt.Sprintf("More than one state: %v", c.state))
	}
	for _, v := range c.state {
		return v
	}
	panic("space/time has unravelled, sorry")
}

type fakeBuildAndDeployer struct {
	t     *testing.T
	calls chan buildAndDeployCall

	buildCount int

	// Set this to simulate a container update that returns the container ID
	// it updated.
	nextBuildContainer container.ID

	nextDeployID model.DeployID

	// Set this to simulate the build failing. Do not set this directly, use fixture.SetNextBuildFailure
	nextBuildFailure error

	buildLogOutput map[model.TargetID]string
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) nextBuildResult(iTarget model.ImageTarget, deployTarget model.TargetSpec) store.BuildResult {
	named := iTarget.Ref.AsNamedOnly()
	nt, _ := reference.WithTag(named, fmt.Sprintf("tilt-%d", b.buildCount))
	containerID := b.nextBuildContainer
	_, isDC := deployTarget.(model.DockerComposeTarget)
	if isDC && containerID == "" {
		// DockerCompose creates a container ID synchronously.
		containerID = container.ID(fmt.Sprintf("dc-%s", path.Base(named.Name())))
	}
	result := store.NewImageBuildResult(iTarget.ID(), nt)
	result.ContainerID = containerID
	return result
}

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, state store.BuildStateSet) (store.BuildResultSet, error) {
	b.buildCount++

	call := buildAndDeployCall{count: b.buildCount, specs: specs, state: state}
	if call.dc().Empty() && call.k8s().Empty() {
		b.t.Fatalf("Invalid call: %+v", call)
	}

	ids := []model.TargetID{}
	for _, spec := range specs {
		id := spec.ID()
		ids = append(ids, id)
		output, ok := b.buildLogOutput[id]
		if ok {
			logger.Get(ctx).Infof(output)
		}
	}

	defer func() {
		// don't update b.calls until the end, to ensure appropriate actions have been dispatched first
		select {
		case b.calls <- call:
		default:
			b.t.Error("writing to fakeBuildAndDeployer would block. either there's a bug or the buffer size needs to be increased")
		}

		logger.Get(ctx).Infof("fake building %s", ids)
	}()

	err := b.nextBuildFailure
	if err != nil {
		b.nextBuildFailure = nil
		return store.BuildResultSet{}, err
	}

	dID := testDeployID
	if b.nextDeployID != 0 {
		dID = b.nextDeployID
		b.nextDeployID = 0
	}

	deployIDActions := NewDeployIDActionsForTargets(ids, dID)
	for _, a := range deployIDActions {
		st.Dispatch(a)
	}

	result := store.BuildResultSet{}
	for _, iTarget := range extractImageTargets(specs) {
		var deployTarget model.TargetSpec
		if !call.dc().Empty() {
			if isImageDeployedToDC(iTarget, call.dc()) {
				deployTarget = call.dc()
			}
		} else {
			if isImageDeployedToK8s(iTarget, []model.K8sTarget{call.k8s()}) {
				deployTarget = call.k8s()
			}
		}

		result[iTarget.ID()] = b.nextBuildResult(iTarget, deployTarget)
	}

	if !call.dc().Empty() {
		result[call.dc().ID()] = store.NewContainerBuildResult(call.dc().ID(), b.nextBuildContainer)
	}

	b.nextBuildContainer = ""

	return result, nil
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{
		t:              t,
		calls:          make(chan buildAndDeployCall, 20),
		buildLogOutput: make(map[model.TargetID]string),
	}
}

func TestUpper_Up(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar", nil)

	gYaml := k8s.NewK8sOnlyManifestForTesting(model.ManifestName("my-global_yaml"),
		testyaml.BlorgBackendYAML)
	err := f.upper.Init(f.ctx, InitAction{
		Manifests:          []model.Manifest{manifest},
		GlobalYAMLManifest: gYaml,
		ExecuteTiltfile:    true,
	})
	close(f.b.calls)
	assert.Nil(t, err)
	var started []model.TargetID
	for call := range f.b.calls {
		started = append(started, call.k8s().ID())
	}
	assert.Equal(t, []model.TargetID{manifest.K8sTarget().ID()}, started)

	state := f.upper.store.RLockState()
	defer f.upper.store.RUnlockState()
	lines := strings.Split(state.ManifestTargets[manifest.Name].Status().LastBuild().Log.String(), "\n")
	assertLineMatches(t, lines, regexp.MustCompile("fake building .*foobar"))
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
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	f.timerMaker.maxTimerLock.Lock()
	call := f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	fileRelPath := "fdas"
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath(fileRelPath)}

	call = f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.oneState().LastImageAsString())
	fileAbsPath, err := filepath.Abs(fileRelPath)
	if err != nil {
		t.Errorf("error making abs path of %v: %v", fileRelPath, err)
	}
	assert.Equal(t, []string{fileAbsPath}, call.oneState().FilesChanged())

	f.withManifestState("foobar", func(ms store.ManifestState) {
		assert.True(t, ms.LastBuild().Reason.Has(model.BuildReasonFlagMountFiles))
	})

	err = f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChanges(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	f.timerMaker.maxTimerLock.Lock()
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	f.timerMaker.restTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath(fileRelPath)}
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.restTimerLock.Unlock()

	call = f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())

	var fileAbsPaths []string
	for _, fileRelPath := range fileRelPaths {
		fileAbsPath, err := filepath.Abs(fileRelPath)
		if err != nil {
			t.Errorf("error making abs path of %v: %v", fileRelPath, err)
		}
		fileAbsPaths = append(fileAbsPaths, fileAbsPath)
	}
	assert.Equal(t, fileAbsPaths, call.oneState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChangesHitMaxTimeout(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	f.timerMaker.maxTimerLock.Lock()
	f.timerMaker.restTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath(fileRelPath)}
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.maxTimerLock.Unlock()

	call = f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())

	var fileAbsPaths []string
	for _, fileRelPath := range fileRelPaths {
		fileAbsPath, err := filepath.Abs(fileRelPath)
		if err != nil {
			t.Errorf("error making abs path of %v: %v", fileRelPath, err)
		}
		fileAbsPaths = append(fileAbsPaths, fileAbsPath)
	}
	assert.Equal(t, fileAbsPaths, call.oneState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.SetNextBuildFailure(errors.New("Build failed"))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("a.go")}

	call = f.nextCall()
	assert.True(t, call.oneState().IsEmpty())
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneState().FilesChanged())

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
	assert.True(t, call.oneState().IsEmpty())

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

	err := f.upper.Init(f.ctx, InitAction{Manifests: []model.Manifest{manifest}, ExecuteTiltfile: true})
	expectedErrStr := fmt.Sprintf("Build Failed: %v", buildFailedToken)
	assert.Equal(t, expectedErrStr, err.Error())
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneState().IsEmpty())

	// Simulate a change to a.go that makes the build fail.
	f.SetNextBuildFailure(errors.New("build failed"))
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("a.go")}

	call = f.nextCallComplete("failed build from a.go change")
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.oneState().LastImageAsString())
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneState().FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("b.go")}

	// The next build should treat both a.go and b.go as changed, and build
	// on the last successful result, from before a.go changed.
	call = f.nextCallComplete("build on last successful result")
	assert.Equal(t, []string{f.JoinPath("a.go"), f.JoinPath("b.go")}, call.oneState().FilesChanged())
	assert.Equal(t, "docker.io/library/foobar:tilt-1", call.oneState().LastImageAsString())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestThreeBuilds(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneState().IsEmpty())

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("a.go")}

	call = f.nextCallComplete("second build")
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneState().FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("b.go")}

	call = f.nextCallComplete("third build")
	assert.Equal(t, []string{f.JoinPath("b.go")}, call.oneState().FilesChanged())

	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, 2, len(ms.BuildHistory))
		assert.Equal(t, []string{f.JoinPath("b.go")}, ms.BuildHistory[0].Edits)
		assert.Equal(t, []string{f.JoinPath("a.go")}, ms.BuildHistory[1].Edits)
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestRebuildWithSpuriousChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	// Simulate a change to .#a.go that's a broken symlink.
	realPath := filepath.Join(f.Path(), "a.go")
	tmpPath := filepath.Join(f.Path(), ".#a.go")
	_ = os.Symlink(realPath, tmpPath)

	f.fsWatcher.events <- watch.FileEvent{Path: tmpPath}

	f.assertNoCall()

	f.TouchFiles([]string{realPath})
	f.fsWatcher.events <- watch.FileEvent{Path: realPath}

	call = f.nextCall()
	assert.Equal(t, []string{realPath}, call.oneState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestRebuildDockerfileViaImageBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: with the old manifest
	call := f.nextCall("old manifest")
	assert.Equal(t, `FROM iron/go:prod`, call.image().FastBuildInfo().BaseDockerfile)

	f.WriteConfigFiles("Dockerfile", `FROM iron/go:dev`)

	// Second call: new manifest!
	call = f.nextCall("new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, testyaml.SnackYAMLPostConfig, call.k8s().YAML)

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.oneState().HasImage())

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("random_file.go")}

	// third call: new manifest should persist
	call = f.nextCall("persist new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.image().FastBuildInfo().BaseDockerfile)

	// Unchanged manifest --> we do NOT clear the build state
	assert.True(t, call.oneState().HasImage())

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
	f.WriteFile("Dockerfile1", `FROM iron/go:prod`)
	f.WriteFile("Dockerfile2", `FROM iron/go:prod`)
	f.WriteFile("doggos.yaml", testyaml.DoggosDeploymentYaml)

	f.loadAndStart()

	// First call: with the old manifests
	call := f.nextCall("old manifest (baz)")
	assert.Equal(t, `FROM iron/go:prod`, call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "baz", string(call.k8s().Name))

	call = f.nextCall("old manifest (quux)")
	assert.Equal(t, `FROM iron/go:prod`, call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "quux", string(call.k8s().Name))

	// rewrite the dockerfiles
	f.WriteConfigFiles(
		"Dockerfile1", `FROM iron/go:dev1`,
		"Dockerfile2", "FROM iron/go:dev2")

	// Now with the manifests from the config files
	call = f.nextCall("manifest from config files (baz)")
	assert.Equal(t, `FROM iron/go:dev1`, call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "baz", string(call.k8s().Name))

	call = f.nextCall("manifest from config files (quux)")
	assert.Equal(t, `FROM iron/go:dev2`, call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "quux", string(call.k8s().Name))

	// Now change a dockerfile
	f.WriteConfigFiles("Dockerfile1", `FROM node:10`)

	// Second call: one new manifest!
	call = f.nextCall("changed config file --> new manifest")

	assert.Equal(t, "baz", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{}, call.oneState().FilesChanged())

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.oneState().HasImage())

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

k8s_resource("baz", 'snack.yaml')  # rename "snack" --> "baz"
`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("Dockerfile1", `FROM iron/go:dev1`)
	f.WriteFile("Dockerfile2", `FROM iron/go:dev2`)
	f.WriteFile("doggos.yaml", testyaml.DoggosDeploymentYaml)

	f.loadAndStart()

	// First call: with one resource
	call := f.nextCall("old manifest (baz)")
	assert.Equal(t, "FROM iron/go:dev1", call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "baz", string(call.k8s().Name))

	f.assertNoCall()

	// Now add a second resource
	f.WriteConfigFiles("Tiltfile", `
fast_build("gcr.io/windmill-public-containers/servantes/snack", "Dockerfile1")
fast_build("gcr.io/windmill-public-containers/servantes/doggos", "Dockerfile2")

k8s_resource("baz", 'snack.yaml')  # rename "snack" --> "baz"
k8s_resource("quux", 'doggos.yaml')  # rename "doggos" --> "quux"
`)

	// Expect a build of quux, the new resource
	call = f.nextCall("changed config file --> new manifest")
	assert.Equal(t, "quux", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{}, call.oneState().FilesChanged())

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestNoOpChangeToDockerfile(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
r = local_git_repo('.')
fast_build('gcr.io/windmill-public-containers/servantes/snack', 'Dockerfile') \
  .add(r.path('src'), '.')
k8s_resource('foobar', 'snack.yaml')`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("src/main.go", "hello")

	f.loadAndStart()

	// First call: with the old manifests
	call := f.nextCall("old manifests")
	assert.Equal(t, "FROM iron/go:dev1", call.image().FastBuildInfo().BaseDockerfile)
	assert.Equal(t, "foobar", string(call.k8s().Name))

	f.WriteConfigFiles("Dockerfile", `FROM iron/go:dev1`)

	// The dockerfile hasn't changed, so there shouldn't be any builds.
	f.assertNoCall()

	changed := f.WriteFile("src/main.go", "goodbye")
	f.fsWatcher.events <- watch.FileEvent{Path: changed}

	// Second call: Editing the Dockerfile means we have to reevaluate the Tiltfile.
	// Editing the random file means we have to do a rebuild. BUT! The Dockerfile
	// hasn't changed, so the manifest hasn't changed, so we can do an incremental build.
	call = f.nextCall("incremental build despite edited config file")
	assert.Equal(t, "foobar", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{
		f.JoinPath("src/main.go"),
	}, call.oneState().FilesChanged())

	// Unchanged manifest --> we do NOT clear the build state
	assert.True(t, call.oneState().HasImage())

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

	f.loadAndStart()

	// First call: init
	call := f.nextCall("old manifest")
	assert.Equal(t, `FROM iron/go:dev`, call.image().FastBuildInfo().BaseDockerfile)

	// Second call: error!
	f.WriteConfigFiles("Tiltfile", "borken")
	f.assertNoCall("Tiltfile error should prevent BuildAndDeploy from being called")

	// Third call: fix
	f.WriteConfigFiles("Tiltfile", simpleTiltfile,
		"Dockerfile", `FROM iron/go:dev2`)

	call = f.nextCall("fixed broken config")
	assert.Equal(t, "FROM iron/go:dev2", call.image().FastBuildInfo().BaseDockerfile)
	assert.False(t, call.oneState().HasImage()) // we cleared the previous build state to force an image build
	f.WaitUntil("manifest definition order hasn't changed", func(state store.EngineState) bool {
		return len(state.ManifestDefinitionOrder) == 1
	})
	f.WaitUntilManifestState("LastError was cleared", "foobar", func(ms store.ManifestState) bool {
		return ms.LastBuild().Error == nil
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
		return st.LastTiltfileError() != nil
	})

	f.withState(func(es store.EngineState) {
		assert.Equal(t, "", nextManifestNameToBuild(es).String())
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
		return st.LastTiltfileError() != nil
	})

	// Third call: put Tiltfile back. No change to manifest or to mounted files, so expect no build.
	f.WriteConfigFiles("Tiltfile", origTiltfile)
	f.WaitUntil("state is restored", func(st store.EngineState) bool {
		return st.LastTiltfileError() == nil
	})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, "", nextManifestNameToBuild(state).String())
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

	name := model.ManifestName("foobar")
	// Second call: change Tiltfile, break manifest
	f.WriteConfigFiles("Tiltfile", "borken")
	f.WaitUntil("manifest load error", func(st store.EngineState) bool {
		return st.LastTiltfileError() != nil
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
		assert.Equal(t, "", nextManifestNameToBuild(state).String())
		assert.NoError(t, state.LastTiltfileError())
	})

	f.withManifestTarget(name, func(mt store.ManifestTarget) {
		expectedSteps := []model.Step{{
			Cmd:           model.ToShellCmd("changed"),
			BaseDirectory: f.Path(),
		}}
		assert.Equal(t, expectedSteps, mt.Manifest.ImageTargetAt(0).FastBuildInfo().Steps)
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
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithBuildDetails(
		model.StaticBuild{
			Dockerfile: df,
			BuildPath:  f.Path(),
		}))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneState().IsEmpty())

	// Simulate a change to main.go
	mainPath := filepath.Join(f.Path(), "main.go")
	f.fsWatcher.events <- watch.FileEvent{Path: mainPath}

	// Check that this triggered a rebuild.
	call = f.nextCallComplete("rebuild triggered")
	assert.Equal(t, []string{mainPath}, call.oneState().FilesChanged())

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
	assert.True(t, call.oneState().IsEmpty())

	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0
	})

	err = f.Stop()
	assert.Equal(t, nil, err)

	assert.Equal(t, 2, len(f.hud.LastView.Resources))
	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, manifest.Name, model.ManifestName(rv.Name))
	assert.Equal(t, ".", rv.DirectoriesWatched[0])
	f.assertAllBuildsConsumed()
}

func (f *testFixture) testPod(podID string, manifestName string, phase string, cID string, creationTime time.Time) *v1.Pod {
	return f.testPodWithDeployID(podID, manifestName, phase, cID, creationTime, testDeployID)
}

func (f *testFixture) testPodWithDeployID(podID string, manifestName string, phase string, cID string, creationTime time.Time, deployID model.DeployID) *v1.Pod {
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
			Labels: map[string]string{
				k8s.ManifestNameLabel: manifestName,
				k8s.TiltDeployIDLabel: deployID.String(),
			},
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
	assert.True(t, call.oneState().IsEmpty())

	f.podEvent(f.testPod("my-pod", "foobar", "CrashLoopBackOff", testContainer, time.Now()))

	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].K8SInfo().PodName == "my-pod"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-pod", rv.K8SInfo().PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.K8SInfo().PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	past := time.Now().Add(-time.Minute)
	now := time.Now()
	deployIDPast := model.DeployID(111)
	deployIDNow := model.DeployID(999)
	podAPast := f.testPodWithDeployID("pod-a", "fe", "Running", testContainer, past, deployIDPast)
	podBPast := f.testPodWithDeployID("pod-b", "fe", "Running", testContainer, past, deployIDPast)
	podANow := f.testPodWithDeployID("pod-a", "fe", "Running", testContainer, now, deployIDNow)
	podBNow := f.testPodWithDeployID("pod-b", "fe", "Running", testContainer, now, deployIDNow)
	podCNow := f.testPodWithDeployID("pod-b", "fe", "Running", testContainer, now, deployIDNow)
	podCNowDeleting := f.testPodWithDeployID("pod-c", "fe", "Running", testContainer, now, deployIDNow)
	podCNowDeleting.DeletionTimestamp = &metav1.Time{Time: now}

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
			f.b.nextDeployID = deployIDNow
			f.Start([]model.Manifest{manifest}, true)

			call := f.nextCall()
			assert.True(t, call.oneState().IsEmpty())
			f.WaitUntilManifestState("deployID set", "fe", func(ms store.ManifestState) bool {
				return ms.DeployID == deployIDNow
			})

			for _, pod := range order {
				f.podEvent(pod)
			}

			f.upper.store.Dispatch(PodLogAction{
				ManifestName: "fe",
				PodID:        k8s.PodIDFromPod(podBNow),
				logEvent:     newLogEvent([]byte("pod b log\n")),
			})

			f.WaitUntilManifestState("pod log seen", "fe", func(ms store.ManifestState) bool {
				return strings.Contains(ms.MostRecentPod().Log(), "pod b log")
			})

			f.withManifestState("fe", func(ms store.ManifestState) {
				if assert.Equal(t, 2, ms.PodSet.Len()) {
					assert.Equal(t, now.String(), ms.PodSet.Pods["pod-a"].StartedAt.String())
					assert.Equal(t, now.String(), ms.PodSet.Pods["pod-b"].StartedAt.String())
					assert.Equal(t, deployIDNow, ms.PodSet.DeployID)
				}
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
	f.WaitUntilManifestState("image appears", "foobar", func(ms store.ManifestState) bool {
		ref = ms.BuildStatus(manifest.ImageTargetAt(0).ID()).LastSuccessfulResult.Image
		return ref != nil
	})

	pod := f.testPod("my-pod", "foobar", "Running", testContainer, time.Now())
	pod.Status = k8s.FakePodStatus(ref, "Running")
	pod.Status.ContainerStatuses[0].ContainerID = ""
	pod.Spec = k8s.FakePodSpec(ref)

	f.podEvent(pod)

	podState := store.Pod{}
	f.WaitUntilManifestState("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.MostRecentPod()
		return podState.PodID == "my-pod"
	})

	assert.Equal(t, "", string(podState.ContainerID))
	assert.Equal(t, "main", string(podState.ContainerName))
	assert.Equal(t, []int32{8080}, podState.ContainerPorts)

	err := f.Stop()
	assert.Nil(t, err)
}

func TestPodEventContainerStatusWithoutImage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := model.Manifest{
		Name: model.ManifestName("foobar"),
	}.WithDeployTarget(model.K8sTarget{
		YAML: "fake-yaml",
	})
	deployID := model.DeployID(123)
	f.b.nextDeployID = deployID
	ref := container.MustParseNamedTagged("dockerhub/we-didnt-build-this:foo")
	f.Start([]model.Manifest{manifest}, true)

	f.WaitUntilManifestState("first build complete", "foobar", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) > 0
	})

	pod := f.testPodWithDeployID("my-pod", "foobar", "Running", testContainer, time.Now(), deployID)
	pod.Status = k8s.FakePodStatus(ref, "Running")

	// If we have no image target to match container status by image ref,
	// we should just take the first one, i.e. this one
	pod.Status.ContainerStatuses[0].Name = "first-container"
	pod.Status.ContainerStatuses[0].ContainerID = "docker://great-container-id"

	pod.Spec = v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "second-container",
				Image: "gcr.io/windmill-public-containers/tilt-synclet:latest",
				Ports: []v1.ContainerPort{{ContainerPort: 9999}},
			},
			// we match container spec by NAME, so we'll get this one even tho it comes second.
			{
				Name:  "first-container",
				Image: ref.Name(),
				Ports: []v1.ContainerPort{{ContainerPort: 8080}},
			},
		},
	}

	f.podEvent(pod)

	podState := store.Pod{}
	f.WaitUntilManifestState("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.MostRecentPod()
		return podState.PodID == "my-pod"
	})

	// If we have no image target to match container by image ref, we just take the first one
	assert.Equal(t, "great-container-id", string(podState.ContainerID))
	assert.Equal(t, "first-container", string(podState.ContainerName))
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
	f.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))

	f.WaitUntil("builds ready & changed file recorded", func(st store.EngineState) bool {
		ms, _ := st.ManifestState(manifest.Name)
		return nextManifestNameToBuild(st) == manifest.Name && ms.HasPendingFileChanges()
	})
	f.store.Dispatch(BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    time.Now(),
	})
	f.store.Dispatch(BuildCompleteAction{
		Result: containerResultSet(manifest, "theOriginalContainer"),
	})
	f.setDeployIDForManifest(manifest, testDeployID)

	f.WaitUntil("nothing waiting for build", func(st store.EngineState) bool {
		return st.CompletedBuildCount == 1 && nextManifestNameToBuild(st) == ""
	})

	f.podEvent(f.testPod("mypod", "foobar", "Running", "myfunnycontainerid", time.Now()))

	f.WaitUntilManifestState("NeedsRebuildFromCrash set to True", "foobar", func(ms store.ManifestState) bool {
		return ms.NeedsRebuildFromCrash
	})

	f.WaitUntil("manifest queued for build b/c it's crashing", func(st store.EngineState) bool {
		return nextManifestNameToBuild(st) == manifest.Name
	})
}

func TestPodUnexpectedContainerStartsImageBuildOutOfOrderEvents(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)

	// Start a fake build
	f.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))
	f.WaitUntil("builds ready & changed file recorded", func(st store.EngineState) bool {
		ms, _ := st.ManifestState(manifest.Name)
		return nextManifestNameToBuild(st) == manifest.Name && ms.HasPendingFileChanges()
	})
	f.store.Dispatch(BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    time.Now(),
	})
	f.setDeployIDForManifest(manifest, testDeployID)

	// Simulate k8s restarting the container due to a crash.
	f.podEvent(f.testPod("mypod", "foobar", "Running", "myfunnycontainerid", time.Now()))
	// ...and finish the build. Even though this action comes in AFTER the pod
	// event w/ unexpected container,  we should still be able to detect the mismatch.
	f.store.Dispatch(BuildCompleteAction{
		Result: containerResultSet(manifest, "theOriginalContainer"),
	})

	f.WaitUntilManifestState("NeedsRebuildFromCrash set to True", "foobar", func(ms store.ManifestState) bool {
		return ms.NeedsRebuildFromCrash
	})
	f.WaitUntil("manifest queued for build b/c it's crashing", func(st store.EngineState) bool {
		return nextManifestNameToBuild(st) == manifest.Name
	})
}

func TestPodUnexpectedContainerAfterSuccessfulUpdate(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String(), []model.Mount{mount})

	f.Start([]model.Manifest{manifest}, true)

	// Start and end a normal build
	f.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))
	f.WaitUntil("builds ready & changed file recorded", func(st store.EngineState) bool {
		ms, _ := st.ManifestState(manifest.Name)
		return nextManifestNameToBuild(st) == manifest.Name && ms.HasPendingFileChanges()
	})

	f.store.Dispatch(BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    time.Now(),
	})
	podStartTime := time.Now()
	f.store.Dispatch(BuildCompleteAction{
		Result: containerResultSet(manifest, "normal-container-id"),
	})
	f.setDeployIDForManifest(manifest, testDeployID)

	f.podEvent(f.testPod("mypod", "foobar", "Running", "normal-container-id", podStartTime))
	f.WaitUntil("nothing waiting for build", func(st store.EngineState) bool {
		return st.CompletedBuildCount == 1 && nextManifestNameToBuild(st) == ""
	})

	// Start another fake build
	f.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))
	f.WaitUntil("waiting for builds to be ready", func(st store.EngineState) bool {
		return nextManifestNameToBuild(st) == manifest.Name
	})
	f.store.Dispatch(BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    time.Now(),
	})

	// Simulate a pod crash, then a build completion
	f.podEvent(f.testPod("mypod", "foobar", "Running", "funny-container-id", podStartTime))
	f.store.Dispatch(BuildCompleteAction{
		Result: containerResultSet(manifest, "normal-container-id"),
	})

	f.WaitUntilManifestState("NeedsRebuildFromCrash set to True", "foobar", func(ms store.ManifestState) bool {
		return ms.NeedsRebuildFromCrash
	})
	f.WaitUntil("manifest queued for build b/c it's crashing", func(st store.EngineState) bool {
		return nextManifestNameToBuild(st) == manifest.Name
	})
}

func TestPodEventUpdateByTimestamp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	firstCreationTime := time.Now()
	f.podEvent(f.testPod("my-pod", "foobar", "CrashLoopBackOff", testContainer, firstCreationTime))
	f.WaitUntilHUD("hud update 1", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].K8SInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-new-pod", "foobar", "Running", testContainer, firstCreationTime.Add(time.Minute*2)))
	f.WaitUntilHUD("hud update 2", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].K8SInfo().PodStatus == "Running"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-new-pod", rv.K8SInfo().PodName)
	assert.Equal(t, "Running", rv.K8SInfo().PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventUpdateByPodName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("foobar", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete()
	assert.True(t, call.oneState().IsEmpty())

	creationTime := time.Now()
	f.podEvent(f.testPod("my-pod", "foobar", "CrashLoopBackOff", testContainer, creationTime))

	f.WaitUntilHUD("pod crashes", func(view view.View) bool {
		rv := view.Resources[0]
		return rv.K8SInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-pod", "foobar", "Running", testContainer, creationTime))

	f.WaitUntilHUD("pod comes back", func(view view.View) bool {
		rv := view.Resources[0]
		return rv.K8SInfo().PodStatus == "Running"
	})

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-pod", rv.K8SInfo().PodName)
	assert.Equal(t, "Running", rv.K8SInfo().PodStatus)

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
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	creationTime := time.Now()
	f.podEvent(f.testPod("my-new-pod", "foobar", "CrashLoopBackOff", testContainer, creationTime))
	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0 && v.Resources[0].K8SInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-pod", "foobar", "Running", testContainer, creationTime.Add(time.Minute*-1)))
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()

	rv := f.hud.LastView.Resources[0]
	assert.Equal(t, "my-new-pod", rv.K8SInfo().PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.K8SInfo().PodStatus)
}

func TestPodContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	_ = f.nextCall()

	var ref reference.NamedTagged
	f.WaitUntilManifestState("image appears", "fe", func(ms store.ManifestState) bool {
		ref = ms.BuildStatus(manifest.ImageTargetAt(0).ID()).LastSuccessfulResult.Image
		return ref != nil
	})

	startedAt := time.Now()
	f.podEvent(f.testPod("pod-id", "fe", "Running", testContainer, startedAt))
	f.WaitUntilManifestState("pod appears", "fe", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == "pod-id"
	})

	pod := f.testPod("pod-id", "fe", "Running", testContainer, startedAt)
	pod.Spec = k8s.FakePodSpec(ref)
	pod.Status = k8s.FakePodStatus(ref, "Running")
	f.podEvent(pod)

	f.WaitUntilManifestState("container is ready", "fe", func(ms store.ManifestState) bool {
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
	manifest := f.newManifest("foobar", []model.Mount{mount})
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).
		WithDockerignores([]model.Dockerignore{
			{
				LocalPath: f.Path(),
				Contents:  "dignore.txt",
			},
		}))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())

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
	manifest := f.newManifest("foobar", []model.Mount{mount})
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).
		WithRepos([]model.LocalGitRepo{
			{
				LocalPath:         f.Path(),
				GitignoreContents: "gignore.txt",
			},
		}))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())

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

	f.upper.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a.go"))

	f.waitForCompletedBuildCount(2)
	f.podLog(name, "second string")

	f.withManifestState(name, func(ms store.ManifestState) {
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

	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "first string\n", ms.MostRecentPod().Log())
	})

	f.upper.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a.go"))

	f.waitForCompletedBuildCount(2)

	f.withManifestState(name, func(ms store.ManifestState) {
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
	f.withManifestState(name, func(ms store.ManifestState) {
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
	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "second string\n", ms.MostRecentPod().Log())
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func testService(serviceName string, manifestName string, ip string, port int) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: map[string]string{k8s.ManifestNameLabel: manifestName},
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

	f.WaitUntilManifestState("lb updated", "foobar", func(ms store.ManifestState) bool {
		return len(ms.LBs) > 0
	})

	err := f.Stop()
	assert.NoError(t, err)

	ms, _ := f.upper.store.RLockState().ManifestState(manifest.Name)
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

	f.WaitUntilManifestState("lb url added", "foobar", func(ms store.ManifestState) bool {
		url := ms.LBs["myservice"]
		if url == nil {
			return false
		}
		return "http://1.2.3.4:8080/" == url.String()
	})

	svc = testService("myservice", "foobar", "1.2.3.4", 8080)
	svc.Status = v1.ServiceStatus{}
	dispatchServiceChange(f.store, svc, "")

	f.WaitUntilManifestState("lb url removed", "foobar", func(ms store.ManifestState) bool {
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
	ym := k8s.NewK8sOnlyManifestForTesting("global", testyaml.BlorgBackendYAML)
	state.GlobalYAML = ym
	f.store.RUnlockState()
	f.Start([]model.Manifest{}, true)
	f.store.Dispatch(InitAction{
		Manifests:          []model.Manifest{},
		GlobalYAMLManifest: ym,
		ExecuteTiltfile:    true,
	})
	f.WaitUntil("global YAML manifest gets set on init", func(st store.EngineState) bool {
		return st.GlobalYAML.K8sTarget().YAML == testyaml.BlorgBackendYAML
	})

	newYM := k8s.NewK8sOnlyManifestForTesting("global", testyaml.BlorgJobYAML)
	f.store.Dispatch(ConfigsReloadedAction{
		GlobalYAML: newYM,
	})

	f.WaitUntil("global YAML manifest gets updated", func(st store.EngineState) bool {
		return st.GlobalYAML.K8sTarget().YAML == testyaml.BlorgJobYAML
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

	f.WaitUntilManifest("has new mounts", "mani1", func(mt store.ManifestTarget) bool {
		return len(mt.Manifest.ImageTargetAt(0).FastBuildInfo().Mounts) == 2
	})

	f.PollUntil("watches set up", func() bool {
		watches, ok := f.fwm.targetWatches[m2.ImageTargetAt(0).ID()]
		if !ok {
			return false
		}
		return len(watches.target.Dependencies()) == 2
	})
}

func TestNewConfigsAreWatchedAfterFailure(t *testing.T) {
	f := newTestFixture(t)
	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	name := model.ManifestName("foo")
	m := f.newManifest(name.String(), []model.Mount{mount})
	f.Start([]model.Manifest{m}, true)
	f.WriteConfigFiles("Tiltfile", "read_file('foo.txt')")
	f.WaitUntil("foo.txt is a config file", func(state store.EngineState) bool {
		for _, s := range state.ConfigFiles {
			if s == f.JoinPath("foo.txt") {
				return true
			}
		}
		return false
	})
}

func TestDockerComposeUp(t *testing.T) {
	f := newTestFixture(t)
	redis, server := f.setupDCFixture()

	f.Start([]model.Manifest{redis, server}, true)
	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())
	assert.False(t, call.dc().ID().Empty())
	assert.Equal(t, redis.DockerComposeTarget().ID(), call.dc().ID())
	call = f.nextCall()
	assert.True(t, call.oneState().IsEmpty())
	assert.False(t, call.dc().ID().Empty())
	assert.Equal(t, server.DockerComposeTarget().ID(), call.dc().ID())
}

func TestDockerComposeRedeployFromFileChange(t *testing.T) {
	f := newTestFixture(t)
	_, m := f.setupDCFixture()

	f.Start([]model.Manifest{m}, true)

	// Initial build
	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	// Change a file -- should trigger build
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("package.json")}
	call = f.nextCall()
	assert.Equal(t, []string{f.JoinPath("package.json")}, call.oneState().FilesChanged())
}

// TODO(maia): TestDockerComposeEditConfigFiles once DC manifests load faster (http://bit.ly/2RBX4g5)

func TestDockerComposeEventSetsStatus(t *testing.T) {
	f := newTestFixture(t)
	_, m := f.setupDCFixture()

	f.Start([]model.Manifest{m}, true)
	f.waitForCompletedBuildCount(1)

	// Send event corresponding to status = "In Progress"
	err := f.dcc.SendEvent(dcContainerEvtForManifest(m, dockercompose.ActionCreate))
	if err != nil {
		f.T().Fatal(err)
	}

	f.WaitUntilManifestState("resource status = 'In Progress'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCResourceState().Status == dockercompose.StatusInProg
	})

	beforeStart := time.Now()

	// Send event corresponding to status = "OK"
	err = f.dcc.SendEvent(dcContainerEvtForManifest(m, dockercompose.ActionStart))
	if err != nil {
		f.T().Fatal(err)
	}

	f.WaitUntilManifestState("resource status = 'OK'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCResourceState().Status == dockercompose.StatusUp
	})

	f.withManifestState(m.ManifestName(), func(ms store.ManifestState) {
		assert.True(t, ms.DCResourceState().StartTime.After(beforeStart))

	})

	// An event unrelated to status shouldn't change the status
	err = f.dcc.SendEvent(dcContainerEvtForManifest(m, dockercompose.ActionExecCreate))
	if err != nil {
		f.T().Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)
	f.WaitUntilManifestState("resource status = 'OK'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCResourceState().Status == dockercompose.StatusUp
	})
}

func TestDockerComposeStartsEventWatcher(t *testing.T) {
	f := newTestFixture(t)
	_, m := f.setupDCFixture()

	// Actual behavior is that we init with zero manifests, and add in manifests
	// after Tiltfile loads. Mimic that here.
	f.Start([]model.Manifest{}, true)
	time.Sleep(10 * time.Millisecond)

	f.store.Dispatch(ConfigsReloadedAction{Manifests: []model.Manifest{m}})
	f.waitForCompletedBuildCount(1)

	// Is DockerComposeEventWatcher watching for events??
	err := f.dcc.SendEvent(dcContainerEvtForManifest(m, dockercompose.ActionCreate))
	if err != nil {
		f.T().Fatal(err)
	}

	f.WaitUntilManifestState("resource status = 'In Progress'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCResourceState().Status == dockercompose.StatusInProg
	})
}

func TestDockerComposeRecordsBuildLogs(t *testing.T) {
	f := newTestFixture(t)
	m, _ := f.setupDCFixture()
	expected := "yarn install"
	f.setBuildLogOutput(m.DockerComposeTarget().ID(), expected)

	f.loadAndStart()
	f.waitForCompletedBuildCount(2)

	// recorded in global log
	assert.Contains(t, f.LogLines(), expected)

	// recorded on manifest state
	f.withManifestState(m.ManifestName(), func(st store.ManifestState) {
		assert.Contains(t, st.LastBuild().Log.String(), expected)
	})
}

func TestDockerComposeRecordsRunLogs(t *testing.T) {
	f := newTestFixture(t)
	m, _ := f.setupDCFixture()
	expected := "hello world"
	output := make(chan string, 1)
	output <- expected
	defer close(output)
	f.setDCRunLogOutput(m.DockerComposeTarget(), output)

	f.loadAndStart()
	f.waitForCompletedBuildCount(2)

	f.WaitUntilManifestState("wait until manifest state has a log", m.ManifestName(), func(st store.ManifestState) bool {
		return st.DCResourceState().Log() != ""
	})

	// recorded on manifest state
	f.withManifestState(m.ManifestName(), func(st store.ManifestState) {
		assert.Contains(t, st.DCResourceState().Log(), expected)
	})
}

func TestDockerComposeFiltersRunLogs(t *testing.T) {
	f := newTestFixture(t)
	m, _ := f.setupDCFixture()
	expected := "Attaching to snack\n"
	output := make(chan string, 1)
	output <- expected
	defer close(output)
	f.setDCRunLogOutput(m.DockerComposeTarget(), output)

	f.loadAndStart()
	f.waitForCompletedBuildCount(2)

	// recorded on manifest state
	f.withManifestState(m.ManifestName(), func(st store.ManifestState) {
		assert.NotContains(t, st.DCResourceState().Log(), expected)
	})
}

func TestDockerComposeDetectsCrashes(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := f.setupDCFixture()

	f.loadAndStart()
	f.waitForCompletedBuildCount(2)

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, dockercompose.StatusCrash, st.DCResourceState().Status)
	})

	f.withManifestState(m2.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, dockercompose.StatusCrash, st.DCResourceState().Status)
	})

	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionKill))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionKill))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionDie))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionStop))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionRename))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionCreate))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionStart))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionDie))

	f.WaitUntilManifestState("has a status", m1.ManifestName(), func(st store.ManifestState) bool {
		return st.DCResourceState().Status != ""
	})

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		assert.Equal(t, dockercompose.StatusCrash, st.DCResourceState().Status)
	})

	f.withManifestState(m2.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, dockercompose.StatusCrash, st.DCResourceState().Status)
	})

	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionKill))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionKill))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionDie))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionStop))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionRename))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionCreate))
	f.dcc.SendEvent(dcContainerEvtForManifest(m1, dockercompose.ActionStart))

	f.WaitUntilManifestState("is not crashing", m1.ManifestName(), func(st store.ManifestState) bool {
		return st.DCResourceState().Status != dockercompose.StatusCrash
	})

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, dockercompose.StatusCrash, st.DCResourceState().Status)
	})
}

func TestDockerComposeBuildCompletedSetsStatusToUpIfSuccessful(t *testing.T) {
	f := newTestFixture(t)
	m1, _ := f.setupDCFixture()

	expected := container.ID("aaaaaa")
	f.b.nextBuildContainer = expected
	f.loadAndStart()

	f.waitForCompletedBuildCount(2)

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		state, ok := st.ResourceState.(dockercompose.State)
		if !ok {
			t.Fatal("expected ResourceState to be docker compose, but it wasn't")
		}
		assert.Equal(t, expected, state.ContainerID)
		assert.Equal(t, dockercompose.StatusUp, state.Status)
	})
}

func TestDockerComposeBuildCompletedDoesntSetStatusIfNotSuccessful(t *testing.T) {
	f := newTestFixture(t)
	m1, _ := f.setupDCFixture()

	f.loadAndStart()

	f.waitForCompletedBuildCount(2)

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		state, ok := st.ResourceState.(dockercompose.State)
		if !ok {
			t.Fatal("expected ResourceState to be docker compose, but it wasn't")
		}
		assert.Empty(t, state.ContainerID)
		assert.Empty(t, state.Status)
	})
}

func TestEmptyTiltfile(t *testing.T) {
	f := newTestFixture(t)
	f.WriteFile("Tiltfile", "")
	go f.upper.Start(f.ctx, []string{}, false, model.TriggerAuto, "Tiltfile", true)
	f.WaitUntil("build is set", func(st store.EngineState) bool {
		return !st.LastTiltfileBuild.Empty()
	})
	f.withState(func(st store.EngineState) {
		assert.Contains(t, st.LastTiltfileBuild.Error.Error(), "No resources found. Check out ")
	})
}

func TestWatchManifestsWithCommonAncestor(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithCommonAncestor(f)
	f.Start([]model.Manifest{m1, m2}, true)

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	f.WriteFile("common/a.txt", "hello world")
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("common/a.txt")}

	f.waitForCompletedBuildCount(4)

	// Make sure that both builds are triggered, and that they
	// are triggered in a particular order.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build2")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

}

func TestConfigChangeThatChangesManifestIsIncludedInManifestsChangedFile(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	tiltfile := `
docker_build('gcr.io/windmill-public-containers/servantes/snack', '.')
k8s_yaml('snack.yaml')`
	f.WriteFile("Tiltfile", tiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	f.waitForCompletedBuildCount(1)

	f.WriteFile("snack.yaml", testyaml.SnackYAMLPostConfig)
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("snack.yaml")}

	f.waitForCompletedBuildCount(2)
	f.withManifestState(model.ManifestName("snack"), func(ms store.ManifestState) {
		assert.Equal(t, []string{f.JoinPath("snack.yaml")}, ms.LastBuild().Edits)
	})

	f.WriteFile("Dockerfile", `FROM iron/go:foobar`)
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("Dockerfile")}

	f.waitForCompletedBuildCount(3)
	f.withManifestState(model.ManifestName("snack"), func(ms store.ManifestState) {
		assert.Equal(t, []string{f.JoinPath("Dockerfile")}, ms.LastBuild().Edits)
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
	docker                *docker.FakeClient
	hud                   *hud.FakeHud
	createManifestsResult chan error
	log                   *bufsync.ThreadSafeBuffer
	store                 *store.Store
	pod                   *v1.Pod
	bc                    *BuildController
	fwm                   *WatchManager
	cc                    *ConfigsController
	dcc                   *dockercompose.FakeDCClient
	tfl                   tiltfile.TiltfileLoader

	onchangeCh chan bool
}

func newTestFixture(t *testing.T) *testFixture {
	f := tempdir.NewTempDirFixture(t)
	watcher := newFakeMultiWatcher()
	b := newFakeBuildAndDeployer(t)

	timerMaker := makeFakeTimerMaker(t)

	dockerClient := docker.NewFakeClient()
	reaper := build.NewImageReaper(dockerClient)

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

	err := os.Chdir(f.Path())
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(f.JoinPath(".git"), os.FileMode(0777))
	if err != nil {
		t.Fatal(err)
	}

	fwm := NewWatchManager(watcher.newSub, timerMaker.maker())
	pfc := NewPortForwardController(k8s)
	ic := NewImageController(reaper)
	gybc := NewGlobalYAMLBuildController(k8s)
	an := analytics.NewMemoryAnalytics()
	ar := ProvideAnalyticsReporter(an, st)

	// TODO(nick): Why does this test use two different docker compose clients???
	fakeDcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	realDcc := dockercompose.NewDockerComposeClient(docker.Env{})

	tfl := tiltfile.ProvideTiltfileLoader(an, realDcc)
	cc := NewConfigsController(tfl)
	dcw := NewDockerComposeEventWatcher(fakeDcc)
	dclm := NewDockerComposeLogManager(fakeDcc)
	pm := NewProfilerManager()
	sCli := synclet.NewFakeSyncletClient()
	sm := NewSyncletManagerForTests(k8s, sCli)
	hudsc := server.ProvideHeadsUpServerController(0, server.HeadsUpServer{}, nil)
	upper := NewUpper(ctx, fakeHud, pw, sw, st, plm, pfc, fwm, bc, ic, gybc, cc, dcw, dclm, pm, sm, ar, hudsc)

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
		docker:         dockerClient,
		hud:            fakeHud,
		log:            log,
		store:          st,
		bc:             bc,
		onchangeCh:     fSub.ch,
		fwm:            fwm,
		cc:             cc,
		dcc:            fakeDcc,
		tfl:            tfl,
	}
}

func (f *testFixture) Start(manifests []model.Manifest, watchMounts bool) {
	f.startWithInitManifests(nil, manifests, watchMounts)
}

// Start ONLY the specified manifests and no others (e.g. if additional manifests
// specified later, don't run them. Like running `tilt up <foo, bar>`.
func (f *testFixture) StartOnly(manifests []model.Manifest, watchMounts bool) {
	mNames := make([]model.ManifestName, len(manifests))
	for i, m := range manifests {
		mNames[i] = m.Name
	}
	f.startWithInitManifests(mNames, manifests, watchMounts)
}

// Empty `initManifests` will run start ALL manifests
func (f *testFixture) startWithInitManifests(initManifests []model.ManifestName, manifests []model.Manifest, watchMounts bool) {
	f.Init(InitAction{
		Manifests:       manifests,
		WatchMounts:     watchMounts,
		TiltfilePath:    f.JoinPath("Tiltfile"),
		ExecuteTiltfile: true,
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
		return len(st.ManifestTargets) == len(manifests) && st.WatchMounts == watchMounts
	})

	f.PollUntil("watches set up", func() bool {
		return !watchMounts || len(f.fwm.targetWatches) == len(watchableTargetsForManifests(manifests))
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

func (f *testFixture) setDeployIDForManifest(manifest model.Manifest, dID model.DeployID) {
	action := NewDeployIDAction(manifest.K8sTarget().ID(), dID)
	f.store.Dispatch(action)
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

func (f *testFixture) withManifestTarget(name model.ManifestName, tf func(ms store.ManifestTarget)) {
	f.withState(func(es store.EngineState) {
		mt, ok := es.ManifestTargets[name]
		if !ok {
			f.T().Fatalf("no manifest state for name %s", name)
		}
		tf(*mt)
	})
}

func (f *testFixture) withManifestState(name model.ManifestName, tf func(ms store.ManifestState)) {
	f.withManifestTarget(name, func(mt store.ManifestTarget) {
		tf(*mt.State)
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

func (f *testFixture) WaitUntilManifest(msg string, name model.ManifestName, isDone func(store.ManifestTarget) bool) {
	f.WaitUntil(msg, func(es store.EngineState) bool {
		mt, ok := es.ManifestTargets[model.ManifestName(name)]
		if !ok {
			return false
		}
		return isDone(*mt)
	})
}

func (f *testFixture) WaitUntilManifestState(msg string, name model.ManifestName, isDone func(store.ManifestState) bool) {
	f.WaitUntilManifest(msg, name, func(mt store.ManifestTarget) bool {
		return isDone(*(mt.State))
	})
}

func (f *testFixture) nextCallComplete(msgAndArgs ...interface{}) buildAndDeployCall {
	call := f.nextCall(msgAndArgs...)
	f.waitForCompletedBuildCount(call.count)
	return call
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
		case <-time.After(200 * time.Millisecond):
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
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}

func (f *testFixture) startPod(manifestName model.ManifestName) {
	pID := k8s.PodID("mypod")
	f.pod = f.testPod(pID.String(), manifestName.String(), "Running", testContainer, time.Now())
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifestState("pod appears", manifestName, func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == k8s.PodID(f.pod.Name)
	})
}

func (f *testFixture) podLog(manifestName model.ManifestName, s string) {
	f.upper.store.Dispatch(PodLogAction{
		ManifestName: manifestName,
		PodID:        k8s.PodID(f.pod.Name),
		logEvent:     newLogEvent([]byte(s + "\n")),
	})

	f.WaitUntilManifestState("pod log seen", manifestName, func(ms store.ManifestState) bool {
		return strings.Contains(ms.MostRecentPod().CurrentLog.String(), s)
	})
}

func (f *testFixture) restartPod() {
	restartCount := f.pod.Status.ContainerStatuses[0].RestartCount + 1
	f.pod.Status.ContainerStatuses[0].RestartCount = restartCount
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifestState("pod restart seen", "foobar", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().ContainerRestarts == int(restartCount)
	})
}

func (f *testFixture) notifyAndWaitForPodStatus(pred func(pod store.Pod) bool) {
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifestState("pod status change seen", "foobar", func(state store.ManifestState) bool {
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
	return container.MustParseNamed(manifestName)
}

func (f *testFixture) newManifest(name string, mounts []model.Mount) model.Manifest {
	ref := container.NewRefSelector(f.imageNameForManifest(name))
	return assembleK8sManifest(
		model.Manifest{Name: model.ManifestName(name)},
		model.K8sTarget{YAML: "fake-yaml"},
		model.ImageTarget{Ref: ref}.
			WithBuildDetails(model.FastBuild{
				BaseDockerfile: `from golang:1.10`,
				Mounts:         mounts,
			}))
}

func (f *testFixture) newDCManifest(name string, DCYAMLRaw string, dockerfileContents string) model.Manifest {
	f.WriteFile("docker-compose.yml", DCYAMLRaw)
	return model.Manifest{
		Name: model.ManifestName(name),
	}.WithDeployTarget(model.DockerComposeTarget{
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
	tlr, err := f.tfl.Load(f.ctx, f.JoinPath(tiltfile.FileName), nil)
	if err != nil {
		f.T().Fatal(err)
	}
	f.Start(tlr.Manifests, true)
}

func (f *testFixture) WriteConfigFiles(args ...string) {
	if (len(args) % 2) != 0 {
		f.T().Fatalf("WriteConfigFiles needs an even number of arguments; got %d", len(args))
	}

	filenames := []string{}
	for i := 0; i < len(args); i += 2 {
		filename := f.JoinPath(args[i])
		contents := args[i+1]
		f.WriteFile(filename, contents)
		filenames = append(filenames, filename)

		// Fire an FS event thru the normal pipeline, so that manifests get marked dirty.
		f.fsWatcher.events <- watch.FileEvent{Path: filename}
	}

	// The test harness was written for a time when most tests didn't
	// have a Tiltfile. So
	// 1) Tiltfile execution doesn't happen at test startup
	// 2) Because the Tiltfile isn't executed, ConfigFiles isn't populated
	// 3) Because ConfigFiles isn't populated, ConfigsTargetID watches aren't set up properly
	// So just fire a change action manually.
	f.store.Dispatch(newTargetFilesChangedAction(ConfigsTargetID, filenames...))
}

func (f *testFixture) setupDCFixture() (redis, server model.Manifest) {
	dcp := filepath.Join(originalWD, "testdata", "fixture_docker-config.yml")
	dcpc, err := ioutil.ReadFile(dcp)
	if err != nil {
		f.T().Fatal(err)
	}
	f.WriteFile("docker-compose.yml", string(dcpc))

	dfp := filepath.Join(originalWD, "testdata", "server.dockerfile")
	dfc, err := ioutil.ReadFile(dfp)
	if err != nil {
		f.T().Fatal(err)
	}
	f.WriteFile("Dockerfile", string(dfc))

	f.WriteFile("Tiltfile", `docker_compose('docker-compose.yml')`)

	tlr, err := f.tfl.Load(f.ctx, f.JoinPath("Tiltfile"), nil)
	if err != nil {
		f.T().Fatal(err)
	}

	if len(tlr.Manifests) != 2 {
		f.T().Fatalf("Expected two manifests. Actual: %v", tlr.Manifests)
	}

	return tlr.Manifests[0], tlr.Manifests[1]
}

func (f *testFixture) setBuildLogOutput(id model.TargetID, output string) {
	f.b.buildLogOutput[id] = output
}

func (f *testFixture) setDCRunLogOutput(dc model.DockerComposeTarget, output <-chan string) {
	f.dcc.RunLogOutput[dc.Name] = output
}

type fixtureSub struct {
	ch chan bool
}

func (s fixtureSub) OnChange(ctx context.Context, st store.RStore) {
	s.ch <- true
}

func dcContainerEvtForManifest(m model.Manifest, action dockercompose.Action) dockercompose.Event {
	return dockercompose.Event{
		Type:    dockercompose.TypeContainer,
		Action:  action,
		Service: m.ManifestName().String(),
	}
}

func containerResultSet(manifest model.Manifest, id container.ID) store.BuildResultSet {
	resultSet := store.BuildResultSet{}
	for _, iTarget := range manifest.ImageTargets {
		ref, _ := reference.WithTag(iTarget.Ref.AsNamedOnly(), "deadbeef")
		result := store.NewImageBuildResult(iTarget.ID(), ref)
		result.ContainerID = id
		resultSet[iTarget.ID()] = result
	}
	return resultSet
}

func assertLineMatches(t *testing.T, lines []string, re *regexp.Regexp) {
	for _, line := range lines {
		if re.MatchString(line) {
			return
		}
	}
	t.Fatalf("Expected line to match: %s. Lines: %v", re.String(), lines)
}
