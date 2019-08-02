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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/github"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/podbuilder"
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
docker_build('gcr.io/windmill-public-containers/servantes/snack', '.')
k8s_yaml('snack.yaml')
`
	simpleYAML = testyaml.SnackYaml
)

// represents a single call to `BuildAndDeploy`
type buildAndDeployCall struct {
	count int
	specs []model.TargetSpec
	state store.BuildStateSet
}

func (c buildAndDeployCall) firstImgTarg() model.ImageTarget {
	iTargs := c.imageTargets()
	if len(iTargs) > 0 {
		return iTargs[0]
	}
	return model.ImageTarget{}
}

func (c buildAndDeployCall) imageTargets() []model.ImageTarget {
	targs := make([]model.ImageTarget, 0, len(c.specs))
	for _, spec := range c.specs {
		t, ok := spec.(model.ImageTarget)
		if ok {
			targs = append(targs, t)
		}
	}
	return targs
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

	// Set this to simulate a container update that returns the container IDs
	// it updated.
	nextLiveUpdateContainerIDs []container.ID

	// Inject the container ID of the container started by Docker Compose.
	// If not set, we will auto-generate an ID.
	nextDockerComposeContainerID container.ID

	nextDeployID model.DeployID

	// Set this to simulate the build failing. Do not set this directly, use fixture.SetNextBuildFailure
	nextBuildFailure error

	buildLogOutput map[model.TargetID]string
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) nextBuildResult(iTarget model.ImageTarget, deployTarget model.TargetSpec) store.BuildResult {
	named := iTarget.DeploymentRef
	nt, _ := reference.WithTag(named, fmt.Sprintf("tilt-%d", b.buildCount))

	var result store.BuildResult
	containerIDs := b.nextLiveUpdateContainerIDs
	if len(containerIDs) > 0 {
		result = store.NewLiveUpdateBuildResult(iTarget.ID(), containerIDs)
	} else {
		result = store.NewImageBuildResult(iTarget.ID(), nt)
	}
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

	dID := podbuilder.FakeDeployID
	if b.nextDeployID != 0 {
		dID = b.nextDeployID
		b.nextDeployID = 0
	}

	deployIDActions := NewDeployIDActionsForTargets(ids, dID)
	for _, a := range deployIDActions {
		st.Dispatch(a)
	}

	result := store.BuildResultSet{}
	for _, iTarget := range model.ExtractImageTargets(specs) {
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

	if !call.dc().Empty() && len(b.nextLiveUpdateContainerIDs) == 0 {
		dcContainerID := container.ID(fmt.Sprintf("dc-%s", path.Base(call.dc().ID().Name.String())))
		if b.nextDockerComposeContainerID != "" {
			dcContainerID = b.nextDockerComposeContainerID
		}
		result[call.dc().ID()] = store.NewDockerComposeDeployResult(call.dc().ID(), dcContainerID)
	}

	b.nextLiveUpdateContainerIDs = nil
	b.nextDockerComposeContainerID = ""

	err := b.nextBuildFailure
	b.nextBuildFailure = nil

	return result, err
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
	manifest := f.newManifest("foobar")

	err := f.upper.Init(f.ctx, InitAction{
		Manifests:       []model.Manifest{manifest},
		ExecuteTiltfile: true,
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
}

func TestUpper_WatchFalseNoManifestsExplicitlyNamed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	err := f.upper.Init(f.ctx, InitAction{
		ExecuteTiltfile: false,
		TiltfilePath:    f.JoinPath("Tiltfile"),
		InitManifests:   nil, // equivalent to `tilt up --watch=false` (i.e. not specifying any manifest names)
	})
	close(f.b.calls)

	if err != nil {
		t.Fatal(err)
	}

	var built []model.TargetID
	for call := range f.b.calls {
		built = append(built, call.k8s().ID())
	}
	if assert.Equal(t, 1, len(built)) {
		assert.Equal(t, "snack", built[0].Name.String())
	}
}

func TestUpper_UpWatchError(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
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
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	f.timerMaker.maxTimerLock.Lock()
	call := f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	fileRelPath := "fdas"
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath(fileRelPath))

	call = f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-1", call.oneState().LastImageAsString())
	fileAbsPath := f.JoinPath(fileRelPath)
	assert.Equal(t, []string{fileAbsPath}, call.oneState().FilesChanged())

	f.withManifestState("foobar", func(ms store.ManifestState) {
		assert.True(t, ms.LastBuild().Reason.Has(model.BuildReasonFlagChangedFiles))
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChanges(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	f.timerMaker.maxTimerLock.Lock()
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	f.timerMaker.restTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath(fileRelPath))
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.restTimerLock.Unlock()

	call = f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())

	var fileAbsPaths []string
	for _, fileRelPath := range fileRelPaths {
		fileAbsPaths = append(fileAbsPaths, f.JoinPath(fileRelPath))
	}
	assert.Equal(t, fileAbsPaths, call.oneState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChangesHitMaxTimeout(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	f.timerMaker.maxTimerLock.Lock()
	f.timerMaker.restTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath(fileRelPath))
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.maxTimerLock.Unlock()

	call = f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())

	var fileAbsPaths []string
	for _, fileRelPath := range fileRelPaths {
		fileAbsPaths = append(fileAbsPaths, f.JoinPath(fileRelPath))
	}
	assert.Equal(t, fileAbsPaths, call.oneState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.SetNextBuildFailure(errors.New("Build failed"))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("a.go"))

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
	manifest := f.newManifest("foobar")
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
	manifest := f.newManifest("foobar")
	buildFailedToken := errors.New("doesn't compile")
	f.SetNextBuildFailure(buildFailedToken)

	err := f.upper.Init(f.ctx, InitAction{Manifests: []model.Manifest{manifest}, ExecuteTiltfile: true})
	expectedErrStr := fmt.Sprintf("Build Failed: %v", buildFailedToken)
	assert.Equal(t, expectedErrStr, err.Error())
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneState().IsEmpty())

	// Simulate a change to a.go that makes the build fail.
	f.SetNextBuildFailure(errors.New("build failed"))
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("a.go"))

	call = f.nextCallComplete("failed build from a.go change")
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-1", call.oneState().LastImageAsString())
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneState().FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("b.go"))

	// The next build should treat both a.go and b.go as changed, and build
	// on the last successful result, from before a.go changed.
	call = f.nextCallComplete("build on last successful result")
	assert.Equal(t, []string{f.JoinPath("a.go"), f.JoinPath("b.go")}, call.oneState().FilesChanged())
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-1", call.oneState().LastImageAsString())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestThreeBuilds(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneState().IsEmpty())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("a.go"))

	call = f.nextCallComplete("second build")
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneState().FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("b.go"))

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
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	// Simulate a change to .#a.go that's a broken symlink.
	realPath := filepath.Join(f.Path(), "a.go")
	tmpPath := filepath.Join(f.Path(), ".#a.go")
	_ = os.Symlink(realPath, tmpPath)

	f.fsWatcher.events <- watch.NewFileEvent(tmpPath)

	f.assertNoCall()

	f.TouchFiles([]string{realPath})
	f.fsWatcher.events <- watch.NewFileEvent(realPath)

	call = f.nextCall()
	assert.Equal(t, []string{realPath}, call.oneState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigFileChangeClearsBuildStateToForceImageBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.WriteFile("Tiltfile", `
docker_build('gcr.io/windmill-public-containers/servantes/snack', '.', live_update=[sync('.', '/app')])
k8s_yaml('snack.yaml')
	`)
	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: with the old manifest
	call := f.nextCall("old manifest")
	assert.Equal(t, `FROM iron/go:prod`, call.firstImgTarg().DockerBuildInfo().Dockerfile)

	f.WriteConfigFiles("Dockerfile", `FROM iron/go:dev`)

	// Second call: new manifest!
	call = f.nextCall("new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, testyaml.SnackYAMLPostConfig, call.k8s().YAML)

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.oneState().HasImage())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("random_file.go"))

	// third call: new manifest should persist
	call = f.nextCall("persist new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.firstImgTarg().DockerBuildInfo().Dockerfile)

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
docker_build("gcr.io/windmill-public-containers/servantes/snack", "./snack", dockerfile="Dockerfile1")
docker_build("gcr.io/windmill-public-containers/servantes/doggos", "./doggos", dockerfile="Dockerfile2")

k8s_yaml(['snack.yaml', 'doggos.yaml'])
k8s_resource('snack', new_name='baz')
k8s_resource('doggos', new_name='quux')
`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("Dockerfile1", `FROM iron/go:prod`)
	f.WriteFile("Dockerfile2", `FROM iron/go:prod`)
	f.WriteFile("doggos.yaml", testyaml.DoggosDeploymentYaml)

	f.loadAndStart()

	// First call: with the old manifests
	call := f.nextCall("old manifest (baz)")
	assert.Equal(t, `FROM iron/go:prod`, call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, "baz", string(call.k8s().Name))

	call = f.nextCall("old manifest (quux)")
	assert.Equal(t, `FROM iron/go:prod`, call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, "quux", string(call.k8s().Name))

	// rewrite the dockerfiles
	f.WriteConfigFiles(
		"Dockerfile1", `FROM iron/go:dev1`,
		"Dockerfile2", "FROM iron/go:dev2")

	// Builds triggered by config file changes
	call = f.nextCall("manifest from config files (baz)")
	assert.Equal(t, `FROM iron/go:dev1`, call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, "baz", string(call.k8s().Name))

	call = f.nextCall("manifest from config files (quux)")
	assert.Equal(t, `FROM iron/go:dev2`, call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, "quux", string(call.k8s().Name))

	// Now change (only one) dockerfile
	f.WriteConfigFiles("Dockerfile1", `FROM node:10`)

	// Second call: one new manifest!
	call = f.nextCall("changed config file --> new manifest")

	assert.Equal(t, "baz", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{}, call.oneState().FilesChanged())

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.oneState().HasImage())

	// Importantly the other manifest, quux, is _not_ called -- the DF change didn't affect its manifest
	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestSecondResourceIsBuilt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
docker_build("gcr.io/windmill-public-containers/servantes/snack", "./snack", dockerfile="Dockerfile1")

k8s_yaml('snack.yaml')
k8s_resource('snack', new_name='baz')  # rename "snack" --> "baz"
`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("Dockerfile1", `FROM iron/go:dev1`)
	f.WriteFile("Dockerfile2", `FROM iron/go:dev2`)
	f.WriteFile("doggos.yaml", testyaml.DoggosDeploymentYaml)

	f.loadAndStart()

	// First call: with one resource
	call := f.nextCall("old manifest (baz)")
	assert.Equal(t, "FROM iron/go:dev1", call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, "baz", string(call.k8s().Name))

	f.assertNoCall()

	// Now add a second resource
	f.WriteConfigFiles("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build("gcr.io/windmill-public-containers/servantes/snack", "./snack", dockerfile="Dockerfile1")
docker_build("gcr.io/windmill-public-containers/servantes/doggos", "./doggos", dockerfile="Dockerfile2")

k8s_yaml(['snack.yaml', 'doggos.yaml'])
k8s_resource('snack', new_name='baz')  # rename "snack" --> "baz"
k8s_resource('doggos', new_name='quux')  # rename "doggos" --> "quux"
`)

	// Expect a build of quux, the new resource
	call = f.nextCall("changed config file --> new manifest")
	assert.Equal(t, "quux", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{}, call.oneState().FilesChanged())

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigChange_NoOpChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile')
k8s_yaml('snack.yaml')`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("src/main.go", "hello")

	f.loadAndStart()

	// First call: with the old manifests
	call := f.nextCall("initial call")
	assert.Equal(t, "FROM iron/go:dev1", call.firstImgTarg().DockerBuildInfo().Dockerfile)
	assert.Equal(t, "snack", string(call.k8s().Name))

	// Write same contents to Dockerfile -- an "edit" event for a config file,
	// but it doesn't change the manifest at all.
	f.WriteConfigFiles("Dockerfile", `FROM iron/go:dev1`)
	f.assertNoCall("Dockerfile hasn't changed, so there shouldn't be any builds")

	// Second call: Editing the Dockerfile means we have to reevaluate the Tiltfile.
	// Editing the random file means we have to do a rebuild. BUT! The Dockerfile
	// hasn't changed, so the manifest hasn't changed, so we can do an incremental build.
	changed := f.WriteFile("src/main.go", "goodbye")
	f.fsWatcher.events <- watch.NewFileEvent(changed)

	call = f.nextCall("build from file change")
	assert.Equal(t, "snack", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{
		f.JoinPath("src/main.go"),
	}, call.oneState().FilesChanged())
	assert.True(t, call.oneState().HasImage(), "Unchanged manifest --> we do NOT clear the build state")

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigChange_TiltfileErrorAndFixWithNoChanges(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	origTiltfile := `
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile')
k8s_yaml('snack.yaml')`
	f.WriteFile("Tiltfile", origTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: all is well
	_ = f.nextCall("first call")

	// Second call: change Tiltfile, break manifest
	f.WriteConfigFiles("Tiltfile", "borken")
	f.WaitUntil("tiltfile error set", func(st store.EngineState) bool {
		return st.LastTiltfileError() != nil
	})
	f.assertNoCall("Tiltfile error should prevent BuildAndDeploy from being called")

	// Third call: put Tiltfile back. No change to manifest or to synced files, so expect no build.
	f.WriteConfigFiles("Tiltfile", origTiltfile)
	f.WaitUntil("tiltfile error cleared", func(st store.EngineState) bool {
		return st.LastTiltfileError() == nil
	})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, "", nextManifestNameToBuild(state).String())
	})
}

func TestConfigChange_TiltfileErrorAndFixWithFileChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	tiltfileWithCmd := func(cmd string) string {
		return fmt.Sprintf(`
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile',
    live_update=[
        sync('./src', '/src'),
        run('%s')
    ]
)
k8s_yaml('snack.yaml')
`, cmd)
	}

	f.WriteFile("Tiltfile", tiltfileWithCmd("original"))
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: all is well
	_ = f.nextCall("first call")

	// Second call: change Tiltfile, break manifest
	f.WriteConfigFiles("Tiltfile", "borken")
	f.WaitUntil("tiltfile error set", func(st store.EngineState) bool {
		return st.LastTiltfileError() != nil
	})

	f.assertNoCall("Tiltfile error should prevent BuildAndDeploy from being called")

	// Third call: put Tiltfile back. manifest changed, so expect a build
	f.WriteConfigFiles("Tiltfile", tiltfileWithCmd("changed"))

	call := f.nextCall("fixed broken config and rebuilt manifest")
	assert.False(t, call.oneState().HasImage(),
		"expected this call to have NO image (since we should have cleared it to force an image build)")

	f.WaitUntil("tiltfile error cleared", func(state store.EngineState) bool {
		return state.LastTiltfileError() == nil
	})

	f.withManifestTarget("snack", func(mt store.ManifestTarget) {
		expectedCmd := model.ToShellCmd("changed")
		assert.Equal(t, expectedCmd, mt.Manifest.ImageTargetAt(0).AnyLiveUpdateInfo().RunSteps()[0].Cmd,
			"Tiltfile change should have propagated to manifest")
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigChange_TriggerModeChangePropagatesButDoesntInvalidateBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	origTiltfile := `
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile')
k8s_yaml('snack.yaml')`
	f.WriteFile("Tiltfile", origTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	_ = f.nextCall("initial build")
	f.WaitUntilManifest("manifest has triggerMode = auto (default)", "snack", func(mt store.ManifestTarget) bool {
		return mt.Manifest.TriggerMode == model.TriggerModeAuto
	})

	// Update Tiltfile to change the trigger mode of the manifest
	tiltfileWithTriggerMode := fmt.Sprintf(`%s

trigger_mode(TRIGGER_MODE_MANUAL)`, origTiltfile)
	f.WriteConfigFiles("Tiltfile", tiltfileWithTriggerMode)

	f.assertNoCall("A change to TriggerMode shouldn't trigger an update (doesn't invalidate current build)")
	f.WaitUntilManifest("triggerMode has changed on manifest", "snack", func(mt store.ManifestTarget) bool {
		return mt.Manifest.TriggerMode == model.TriggerModeManual
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigChange_ManifestWithPendingChangesBuildsIfTriggerModeChangedToAuto(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	baseTiltfile := `trigger_mode(%s)
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile')
k8s_yaml('snack.yaml')`
	triggerManualTiltfile := fmt.Sprintf(baseTiltfile, "TRIGGER_MODE_MANUAL")
	f.WriteFile("Tiltfile", triggerManualTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	// First call: with the old manifests
	_ = f.nextCall("initial build")
	var imageTargetID model.TargetID
	f.WaitUntilManifest("manifest has triggerMode = auto (default)", "snack", func(mt store.ManifestTarget) bool {
		imageTargetID = mt.Manifest.ImageTargetAt(0).ID() // grab for later
		return mt.Manifest.TriggerMode == model.TriggerModeManual
	})

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("src/main.go"))
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(imageTargetID).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	// Update Tiltfile to change the trigger mode of the manifest
	triggerAutoTiltfile := fmt.Sprintf(baseTiltfile, "TRIGGER_MODE_AUTO")
	f.WriteConfigFiles("Tiltfile", triggerAutoTiltfile)

	call := f.nextCall("manifest updated b/c it's now TriggerModeAuto")
	assert.True(t, call.oneState().HasImage(),
		"we did NOT clear the build state (b/c a change to Manifest.TriggerMode does NOT invalidate the build")
	f.WaitUntilManifest("triggerMode has changed on manifest", "snack", func(mt store.ManifestTarget) bool {
		return mt.Manifest.TriggerMode == model.TriggerModeAuto
	})
	f.WaitUntil("manifest is no longer in trigger queue", func(st store.EngineState) bool {
		return len(st.TriggerQueue) == 0
	})

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestDockerRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	df := `FROM golang
ADD ./ ./
go build ./...
`
	manifest := f.newManifest("foobar")
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithBuildDetails(
		model.DockerBuild{
			Dockerfile: df,
			BuildPath:  f.Path(),
		}))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneState().IsEmpty())

	// Simulate a change to main.go
	mainPath := filepath.Join(f.Path(), "main.go")
	f.fsWatcher.events <- watch.NewFileEvent(mainPath)

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
	manifest := f.newManifest("foobar")

	f.docker.ImageListCount++

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

	manifest := f.newManifest("foobar")

	f.Start([]model.Manifest{manifest}, true)
	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) > 0
	})

	err := f.Stop()
	assert.Equal(t, nil, err)

	assert.Equal(t, 2, len(f.hud.LastView.Resources))
	assert.Equal(t, view.TiltfileResourceName, f.hud.LastView.Resources[0].Name.String())
	rv := f.hud.LastView.Resources[1]
	assert.Equal(t, manifest.Name, model.ManifestName(rv.Name))
	assert.Equal(t, f.Path(), rv.DirectoriesWatched[0])
	f.assertAllBuildsConsumed()
}

func (f *testFixture) testPod(podID string, manifest model.Manifest, phase string, creationTime time.Time) *v1.Pod {
	return f.testPodWithDeployID(podID, manifest, phase, creationTime, podbuilder.FakeDeployID)
}

func (f *testFixture) testPodWithDeployID(podID string, manifest model.Manifest, phase string, creationTime time.Time, deployID model.DeployID) *v1.Pod {
	return podbuilder.New(f.T(), manifest).
		WithPodID(podID).
		WithPhase(phase).
		WithCreationTime(creationTime).
		WithDeployID(deployID).
		Build()
}

func TestPodEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	f.podEvent(f.testPod("my-pod", manifest, "CrashLoopBackOff", time.Now()))

	f.WaitUntilHUDResource("hud update", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodName == "my-pod"
	})

	rv := f.hudResource("foobar")
	assert.Equal(t, "my-pod", rv.K8sInfo().PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.K8sInfo().PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")

	past := time.Now().Add(-time.Minute)
	now := time.Now()
	deployIDPast := model.DeployID(111)
	deployIDNow := model.DeployID(999)
	podAPast := f.testPodWithDeployID("pod-a", manifest, "Running", past, deployIDPast)
	podBPast := f.testPodWithDeployID("pod-b", manifest, "Running", past, deployIDPast)
	podANow := f.testPodWithDeployID("pod-a", manifest, "Running", now, deployIDNow)
	podBNow := f.testPodWithDeployID("pod-b", manifest, "Running", now, deployIDNow)
	podCNow := f.testPodWithDeployID("pod-b", manifest, "Running", now, deployIDNow)
	podCNowDeleting := f.testPodWithDeployID("pod-c", manifest, "Running", now, deployIDNow)
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
				PodID:    k8s.PodIDFromPod(podBNow),
				LogEvent: store.NewLogEvent("fe", []byte("pod b log\n")),
			})

			f.WaitUntilManifestState("pod log seen", "fe", func(ms store.ManifestState) bool {
				return strings.Contains(ms.MostRecentPod().Log().String(), "pod b log")
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
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	var ref reference.NamedTagged
	f.WaitUntilManifestState("image appears", "foobar", func(ms store.ManifestState) bool {
		ref = ms.BuildStatus(manifest.ImageTargetAt(0).ID()).LastSuccessfulResult.Image
		return ref != nil
	})

	pod := f.testPod("my-pod", manifest, "Running", time.Now())
	pod.Status = k8s.FakePodStatus(ref, "Running")
	pod.Status.ContainerStatuses[0].ContainerID = ""
	pod.Spec = k8s.FakePodSpec(ref)
	f.podEvent(pod)

	podState := store.Pod{}
	f.WaitUntilManifestState("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.MostRecentPod()
		return podState.PodID == "my-pod" && len(podState.Containers) > 0
	})

	container := podState.Containers[0]
	assert.Equal(t, "", string(container.ID))
	assert.Equal(t, "main", string(container.Name))
	assert.Equal(t, []int32{8080}, podState.AllContainerPorts())

	err := f.Stop()
	assert.Nil(t, err)
}

func TestPodEventContainerStatusWithoutImage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := model.Manifest{
		Name: model.ManifestName("foobar"),
	}.WithDeployTarget(model.K8sTarget{
		YAML: SanchoYAML,
	})
	deployID := model.DeployID(123)
	f.b.nextDeployID = deployID
	ref := container.MustParseNamedTagged("dockerhub/we-didnt-build-this:foo")
	f.Start([]model.Manifest{manifest}, true)

	f.WaitUntilManifestState("first build complete", "foobar", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) > 0
	})

	pod := f.testPodWithDeployID("my-pod", manifest, "Running", time.Now(), deployID)
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
		return podState.PodID == "my-pod" && len(podState.Containers) > 0
	})

	// If we have no image target to match container by image ref, we just take the first one
	container := podState.Containers[0]
	assert.Equal(t, "great-container-id", string(container.ID))
	assert.Equal(t, "first-container", string(container.Name))
	assert.Equal(t, []int32{8080}, podState.AllContainerPorts())

	err := f.Stop()
	assert.Nil(t, err)
}

func TestPodUnexpectedContainerStartsImageBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())
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
	f.setDeployIDForManifest(manifest, podbuilder.FakeDeployID)

	f.WaitUntil("nothing waiting for build", func(st store.EngineState) bool {
		return st.CompletedBuildCount == 1 && nextManifestNameToBuild(st) == ""
	})

	f.podEvent(podbuilder.New(t, manifest).WithPodID("mypod").WithContainerID("myfunnycontainerid").Build())

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

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

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
	f.setDeployIDForManifest(manifest, podbuilder.FakeDeployID)

	// Simulate k8s restarting the container due to a crash.
	f.podEvent(podbuilder.New(t, manifest).WithContainerID("myfunnycontainerid").Build())
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

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

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
	f.setDeployIDForManifest(manifest, podbuilder.FakeDeployID)

	f.podEvent(podbuilder.New(t, manifest).
		WithPodID("mypod").
		WithContainerID("normal-container-id").
		WithCreationTime(podStartTime).
		Build())
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
	f.podEvent(podbuilder.New(t, manifest).
		WithPodID("mypod").
		WithContainerID("funny-container-id").
		WithCreationTime(podStartTime).
		Build())
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
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	firstCreationTime := time.Now()
	f.podEvent(f.testPod("my-pod", manifest, "CrashLoopBackOff", firstCreationTime))
	f.WaitUntilHUDResource("hud update crash", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-new-pod", manifest, "Running", firstCreationTime.Add(time.Minute*2)))
	f.WaitUntilHUDResource("hud update running", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "Running"
	})

	rv := f.hudResource("foobar")
	assert.Equal(t, "my-new-pod", rv.K8sInfo().PodName)
	assert.Equal(t, "Running", rv.K8sInfo().PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventUpdateByPodName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCallComplete()
	assert.True(t, call.oneState().IsEmpty())

	creationTime := time.Now()
	f.podEvent(f.testPod("my-pod", manifest, "CrashLoopBackOff", creationTime))

	f.WaitUntilHUDResource("pod crashes", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-pod", manifest, "Running", creationTime))

	f.WaitUntilHUDResource("pod comes back", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "Running"
	})

	rv := f.hudResource("foobar")
	assert.Equal(t, "my-pod", rv.K8sInfo().PodName)
	assert.Equal(t, "Running", rv.K8sInfo().PodStatus)

	err := f.Stop()
	if err != nil {
		t.Fatal(err)
	}

	f.assertAllBuildsConsumed()
}

func TestPodEventIgnoreOlderPod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.True(t, call.oneState().IsEmpty())

	creationTime := time.Now()
	f.podEvent(f.testPod("my-new-pod", manifest, "CrashLoopBackOff", creationTime))
	f.WaitUntilHUDResource("hud update", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(f.testPod("my-pod", manifest, "Running", creationTime.Add(time.Minute*-1)))
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()

	rv := f.hudResource("foobar")
	assert.Equal(t, "my-new-pod", rv.K8sInfo().PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.K8sInfo().PodStatus)
}

func TestPodContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	_ = f.nextCall()

	var ref reference.NamedTagged
	f.WaitUntilManifestState("image appears", "fe", func(ms store.ManifestState) bool {
		ref = ms.BuildStatus(manifest.ImageTargetAt(0).ID()).LastSuccessfulResult.Image
		return ref != nil
	})

	startedAt := time.Now()
	f.podEvent(f.testPod("pod-id", manifest, "Running", startedAt))
	f.WaitUntilManifestState("pod appears", "fe", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == "pod-id"
	})

	pod := f.testPod("pod-id", manifest, "Running", startedAt)
	pod.Spec = k8s.FakePodSpec(ref)
	pod.Status = k8s.FakePodStatus(ref, "Running")
	f.podEvent(pod)

	f.WaitUntilManifestState("container is ready", "fe", func(ms store.ManifestState) bool {
		ports := ms.MostRecentPod().AllContainerPorts()
		return len(ports) == 1 && ports[0] == 8080
	})

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestUpper_WatchDockerIgnoredFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).
		WithDockerignores([]model.Dockerignore{
			{
				LocalPath: f.Path(),
				Contents:  "dignore.txt",
			},
		}))

	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("dignore.txt"))
	f.assertNoCall("event for ignored file should not trigger build")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpper_ShowErrorPodLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(manifest)
	f.podLog(name, "first string")

	f.upper.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a.go"))

	f.waitForCompletedBuildCount(2)
	f.podLog(name, "second string")

	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "second string\n", ms.MostRecentPod().Log().String())
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestBuildResetsPodLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(manifest)
	f.podLog(name, "first string")

	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "first string\n", ms.MostRecentPod().Log().String())
	})

	f.upper.store.Dispatch(newTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a.go"))

	f.waitForCompletedBuildCount(2)

	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "", ms.MostRecentPod().Log().String())
		assert.Equal(t, ms.LastBuild().StartTime, ms.MostRecentPod().UpdateStartTime)
	})
}

func TestUpperPodLogInCrashLoopThirdInstanceStillUp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(manifest)
	f.podLog(name, "first string")
	f.restartPod()
	f.podLog(name, "second string")
	f.restartPod()
	f.podLog(name, "third string")

	// the third instance is still up, so we want to show the log from the last crashed pod plus the log from the current pod
	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "third string\n", ms.MostRecentPod().Log().String())
		assert.Contains(t, ms.CombinedLog.String(), "second string\n")
		assert.Contains(t, ms.CombinedLog.String(), "third string\n")
		assert.Equal(t, ms.CrashLog.String(), "second string\n")
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestUpperPodLogInCrashLoopPodCurrentlyDown(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(manifest)
	f.podLog(name, "first string")
	f.restartPod()
	f.podLog(name, "second string")
	f.pod.Status.ContainerStatuses[0].Ready = false
	f.notifyAndWaitForPodStatus(func(pod store.Pod) bool {
		return !pod.AllContainersReady()
	})

	// The second instance is down, so we don't include the first instance's log
	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, "second string\n", ms.MostRecentPod().Log().String())
	})

	err := f.Stop()
	assert.NoError(t, err)
}

// This tests a bug that led to infinite redeploys:
// 1. Crash rebuild
// 2. Immediately do a container build, before we get the event with the new container ID in (1). This container build
//    should *not* happen in the pre-(1) container ID. Whether it happens in the container from (1) or yields a fresh
//    container build isn't too important
func TestUpperBuildImmediatelyAfterCrashRebuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{podbuilder.FakeContainerID()}
	f.podEvent(f.testPod("pod-id", manifest, "Running", time.Now()))
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.oneState().OneContainerInfo().PodID.String())
	f.waitForCompletedBuildCount(2)
	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagChangedFiles, ms.LastBuild().Reason)
		assert.Equal(t, podbuilder.FakeContainerIDSet(1), ms.LiveUpdatedContainerIDs)
	})

	f.b.nextDeployID = podbuilder.FakeDeployID + 1
	// Restart the pod with a new container id, to simulate a container restart.
	f.podEvent(podbuilder.New(t, manifest).WithPodID("pod-id").WithContainerID("funnyContainerID").Build())
	call = f.nextCall()
	assert.True(t, call.oneState().OneContainerInfo().Empty())
	f.waitForCompletedBuildCount(3)

	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagCrash, ms.LastBuild().Reason)
	})

	// kick off another build
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main2.go"))
	call = f.nextCall()
	// at this point we have not received a pod event for pod that was started by the crash-rebuild,
	// so any known pod info would have to be invalid to use for a build and this BuildState should
	// not have any ContainerInfo
	assert.True(t, call.oneState().OneContainerInfo().Empty())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpperRecordPodWithMultipleContainers(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	pID := k8s.PodID("foobarpod")
	f.pod = f.testPod(pID.String(), manifest, "Running", time.Now())
	f.pod.Status.ContainerStatuses = append(f.pod.Status.ContainerStatuses, v1.ContainerStatus{
		Name:        "sidecar",
		Image:       "sidecar-image",
		Ready:       false,
		ContainerID: "docker://sidecar",
	})

	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifestState("pod appears", name, func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == k8s.PodID(f.pod.Name)
	})

	f.notifyAndWaitForPodStatus(func(pod store.Pod) bool {
		if len(pod.Containers) != 2 {
			return false
		}

		c1 := pod.Containers[0]
		require.Equal(t, container.Name("sancho"), c1.Name)
		require.Equal(t, podbuilder.FakeContainerID(), c1.ID)
		require.True(t, c1.Ready)

		c2 := pod.Containers[1]
		require.Equal(t, container.Name("sidecar"), c2.Name)
		require.Equal(t, container.ID("sidecar"), c2.ID)
		require.False(t, c2.Ready)

		return true
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestUpperProcessOtherContainersIfOneErrors(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	pID := k8s.PodID("foobarpod")
	f.pod = f.testPod(pID.String(), manifest, "Running", time.Now())
	f.pod.Status.ContainerStatuses = append(f.pod.Status.ContainerStatuses, v1.ContainerStatus{
		Name:  "extra1",
		Image: "extra1-image",
		Ready: false,
		// when populating container info for this pod, we'll error when we try to parse
		// this cID -- we should still populate info for the other containers, though.
		ContainerID: "malformed",
	}, v1.ContainerStatus{
		Name:        "extra2",
		Image:       "extra2-image",
		Ready:       false,
		ContainerID: "docker://extra2",
	})

	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifestState("pod appears", name, func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == k8s.PodID(f.pod.Name)
	})

	f.notifyAndWaitForPodStatus(func(pod store.Pod) bool {
		if len(pod.Containers) != 2 {
			return false
		}

		require.Equal(t, container.Name("sancho"), pod.Containers[0].Name)
		require.Equal(t, container.Name("extra2"), pod.Containers[1].Name)

		return true
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

	manifest := f.newManifest("foobar")

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

	manifest := f.newManifest("foobar")

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

	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name))

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	f.startPod(manifest)

	f.podLog(name, "Hello world!\n")

	err := f.Stop()
	assert.NoError(t, err)
}

func TestK8sEventGlobalLogAndManifestLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	entityUID := "someEntity"
	e := entityWithUID(t, testyaml.DoggosDeploymentYaml, entityUID)

	name := model.ManifestName(e.Name())
	manifest := f.newManifest(string(name))

	objRef := v1.ObjectReference{UID: types.UID(entityUID), Name: e.Name()}

	obj := unstructured.Unstructured{}
	obj.SetLabels(map[string]string{k8s.TiltRunIDLabel: k8s.TiltRunID, k8s.ManifestNameLabel: name.String()})
	obj.SetName(objRef.Name)
	f.kClient.GetResources = map[k8s.GetKey]*unstructured.Unstructured{
		k8s.GetKey{Name: e.Name()}: &obj,
	}

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	warnEvt := &v1.Event{
		InvolvedObject: objRef,
		Message:        "something has happened zomg",
		Type:           v1.EventTypeWarning,
	}
	f.kClient.EmitEvent(f.ctx, warnEvt)

	var warnEvts []k8s.EventWithEntity
	f.WaitUntil("event message appears in manifest log", func(st store.EngineState) bool {
		ms, ok := st.ManifestState(name)
		if !ok {
			t.Fatalf("Manifest %s not found in state", name)
		}

		warnEvts = ms.K8sWarnEvents
		combinedLogString := ms.CombinedLog.String()
		return strings.Contains(combinedLogString, "something has happened zomg")
	})

	f.withState(func(st store.EngineState) {
		assert.Contains(t, st.Log.String(), "something has happened zomg", "event message not in global log")
	})

	if assert.Len(t, warnEvts, 1, "expect ms.K8sWarnEvents to contain 1 event") {
		// Make sure we recorded the event on the manifest state
		evt := warnEvts[0]
		assert.Equal(t, "something has happened zomg", evt.Event.Message)
		assert.Equal(t, name.String(), evt.Entity.Name())
	}

	err := f.Stop()
	assert.NoError(t, err)
}

func TestK8sEventNotLoggedIfNoManifestForUID(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	entityUID := "someEntity"
	e := entityWithUID(t, testyaml.DoggosDeploymentYaml, entityUID)

	name := model.ManifestName(e.Name())
	manifest := f.newManifest(string(name))

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	warnEvt := &v1.Event{
		InvolvedObject: v1.ObjectReference{UID: types.UID(entityUID)},
		Message:        "something has happened zomg",
		Type:           v1.EventTypeWarning,
	}
	f.kClient.EmitEvent(f.ctx, warnEvt)

	time.Sleep(10 * time.Millisecond)

	assert.NotContains(t, f.log.String(), "something has happened zomg",
		"should not log event message b/c it doesn't have a UID -> Manifest mapping")
}

func TestK8sEventDoNotLogNormalEvents(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	entityUID := "someEntity"
	e := entityWithUID(t, testyaml.DoggosDeploymentYaml, entityUID)

	name := model.ManifestName(e.Name())
	manifest := f.newManifest(string(name))

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	normalEvt := &v1.Event{
		InvolvedObject: v1.ObjectReference{UID: types.UID(entityUID)},
		Message:        "all systems are go",
		Type:           v1.EventTypeNormal, // we should NOT log this message
	}
	f.kClient.EmitEvent(f.ctx, normalEvt)

	time.Sleep(10 * time.Millisecond)
	f.withManifestState(name, func(ms store.ManifestState) {
		assert.NotContains(t, ms.CombinedLog.String(), "all systems are go",
			"message for event of type 'normal' should not appear in log")

		assert.Len(t, ms.K8sWarnEvents, 0, "expect ms.K8sWarnEvents to be empty")
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestK8sEventLogTimestamp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	st := f.store.LockMutableStateForTesting()
	st.LogTimestamps = true
	f.store.UnlockMutableState()

	entityUID := "someEntity"
	e := entityWithUID(t, testyaml.DoggosDeploymentYaml, entityUID)

	name := model.ManifestName(e.Name())
	manifest := f.newManifest(string(name))

	obj := unstructured.Unstructured{}
	obj.SetLabels(map[string]string{k8s.TiltRunIDLabel: k8s.TiltRunID, k8s.ManifestNameLabel: name.String()})
	f.kClient.GetResources = map[k8s.GetKey]*unstructured.Unstructured{
		k8s.GetKey{}: &obj,
	}

	f.Start([]model.Manifest{manifest}, true)
	f.waitForCompletedBuildCount(1)

	ts := time.Now().Add(time.Hour * 36) // the future, i.e. timestamp that won't otherwise appear in our log

	warnEvt := &v1.Event{
		InvolvedObject: v1.ObjectReference{UID: types.UID(entityUID)},
		Message:        "something has happened zomg",
		LastTimestamp:  metav1.Time{Time: ts},
		Type:           v1.EventTypeWarning,
	}
	f.kClient.EmitEvent(f.ctx, warnEvt)

	tsPrefix := model.TimestampPrefix(ts)
	f.WaitUntil("event message appears in manifest log", func(st store.EngineState) bool {
		ms, ok := st.ManifestState(name)
		if !ok {
			t.Fatalf("Manifest %s not found in state", name)
		}

		l := ms.CombinedLog.String()
		return strings.Contains(l, "something has happened zomg") && strings.Contains(l, string(tsPrefix))
	})

	err := f.Stop()
	assert.NoError(t, err)
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

// This test only makes sense in FastBuild.
// In LiveUpdate, all syncs are captured by the build context,
// so don't need to be watched independently.
func TestNewSyncsAreWatched(t *testing.T) {
	f := newTestFixture(t)
	sync1 := model.Sync{LocalPath: "/go", ContainerPath: "/go"}
	m1 := f.newFastBuildManifest("mani1", []model.Sync{sync1})
	f.Start([]model.Manifest{
		m1,
	}, true)

	f.waitForCompletedBuildCount(1)

	sync2 := model.Sync{LocalPath: "/js", ContainerPath: "/go"}
	m2 := f.newFastBuildManifest("mani1", []model.Sync{sync1, sync2})
	f.store.Dispatch(ConfigsReloadedAction{
		Manifests: []model.Manifest{m2},
	})

	f.WaitUntilManifest("has new syncs", "mani1", func(mt store.ManifestTarget) bool {
		return len(mt.Manifest.ImageTargetAt(0).TopFastBuildInfo().Syncs) == 2
	})

	f.PollUntil("watches set up", func() bool {
		f.fwm.mu.Lock()
		defer f.fwm.mu.Unlock()
		watches, ok := f.fwm.targetWatches[m2.ImageTargetAt(0).ID()]
		if !ok {
			return false
		}
		return len(watches.target.Dependencies()) == 2
	})
}

func TestNewConfigsAreWatchedAfterFailure(t *testing.T) {
	f := newTestFixture(t)
	name := model.ManifestName("foo")
	m := f.newManifest(name.String())
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
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("package.json"))
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
	f.withState(func(st store.EngineState) {
		assert.Contains(t, st.Log.String(), expected)
	})

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
		return !st.DCResourceState().Log().Empty()
	})

	// recorded on manifest state
	f.withManifestState(m.ManifestName(), func(st store.ManifestState) {
		assert.Contains(t, st.DCResourceState().Log().String(), expected)
		assert.Equal(t, 1, strings.Count(st.CombinedLog.String(), expected))
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
		assert.NotContains(t, st.DCResourceState().Log().String(), expected)
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

	f.WaitUntilManifestState("is crashing", m1.ManifestName(), func(st store.ManifestState) bool {
		return st.DCResourceState().Status == dockercompose.StatusCrash
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
		return st.DCResourceState().Status == dockercompose.StatusUp
	})
}

func TestDockerComposeBuildCompletedSetsStatusToUpIfSuccessful(t *testing.T) {
	f := newTestFixture(t)
	m1, _ := f.setupDCFixture()

	expected := container.ID("aaaaaa")
	f.b.nextDockerComposeContainerID = expected
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

func TestEmptyTiltfile(t *testing.T) {
	f := newTestFixture(t)
	f.WriteFile("Tiltfile", "")
	go f.upper.Start(f.ctx, []string{}, model.TiltBuild{}, false, f.JoinPath("Tiltfile"), true, model.SailModeDisabled, analytics.OptIn)
	f.WaitUntil("build is set", func(st store.EngineState) bool {
		return !st.LastTiltfileBuild.Empty()
	})
	f.withState(func(st store.EngineState) {
		assert.Contains(t, st.LastTiltfileBuild.Error.Error(), "No resources found. Check out ")
		assertContainsOnce(t, st.TiltfileCombinedLog.String(), "No resources found. Check out ")
		assertContainsOnce(t, st.LastTiltfileBuild.Log.String(), "No resources found. Check out ")
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
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("common/a.txt"))

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
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("snack.yaml"))

	f.waitForCompletedBuildCount(2)
	f.withManifestState(model.ManifestName("snack"), func(ms store.ManifestState) {
		assert.Equal(t, []string{f.JoinPath("snack.yaml")}, ms.LastBuild().Edits)
	})

	f.WriteFile("Dockerfile", `FROM iron/go:foobar`)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("Dockerfile"))

	f.waitForCompletedBuildCount(3)
	f.withManifestState(model.ManifestName("snack"), func(ms store.ManifestState) {
		assert.Equal(t, []string{f.JoinPath("Dockerfile")}, ms.LastBuild().Edits)
	})
}

func TestTiltVersionCheck(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	versions := []model.TiltBuild{
		{
			Version: "v1000.10.1",
			Date:    "2019-03-11",
		},
		{
			Version: "v1000.10.2",
			Date:    "2019-03-13",
		},
	}

	f.ghc.LatestReleaseErr = nil
	f.ghc.LatestReleaseRet = versions[0]
	f.tiltVersionCheckDelay = time.Millisecond

	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest}, true)

	f.WaitUntil("latest version is updated the first time", func(state store.EngineState) bool {
		return state.LatestTiltBuild == versions[0]
	})

	f.ghc.LatestReleaseRet = versions[1]
	f.WaitUntil("latest version is updated the second time", func(state store.EngineState) bool {
		return state.LatestTiltBuild == versions[1]
	})
}

func TestSetAnalyticsOpt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	opt := func(ia InitAction) InitAction {
		ia.AnalyticsOpt = analytics.OptIn
		return ia
	}

	f.Start([]model.Manifest{}, true, opt)
	f.store.Dispatch(store.AnalyticsOptAction{Opt: analytics.OptOut})
	f.WaitUntil("opted out", func(state store.EngineState) bool {
		return state.AnalyticsOpt == analytics.OptOut
	})

	// if we don't wait for 1 here, it's possible the state flips to out and back to in before the subscriber sees it,
	// and we end up with no events
	f.opter.waitUntilCount(t, 1)

	f.store.Dispatch(store.AnalyticsOptAction{Opt: analytics.OptIn})
	f.WaitUntil("opted in", func(state store.EngineState) bool {
		return state.AnalyticsOpt == analytics.OptIn
	})

	f.opter.waitUntilCount(t, 2)

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, []analytics.Opt{analytics.OptOut, analytics.OptIn}, f.opter.Calls())
}

func TestFeatureFlagsStoredOnState(t *testing.T) {
	f := newTestFixture(t)

	f.Start([]model.Manifest{}, true)

	f.store.Dispatch(ConfigsReloadedAction{Features: map[string]bool{"foo": true}})

	f.WaitUntil("feature is enabled", func(state store.EngineState) bool {
		return state.Features["foo"] == true
	})

	f.store.Dispatch(ConfigsReloadedAction{Features: map[string]bool{"foo": false}})

	f.WaitUntil("feature is disabled", func(state store.EngineState) bool {
		return state.Features["foo"] == false
	})
}

func TestTeamNameStoredOnState(t *testing.T) {
	f := newTestFixture(t)

	f.Start([]model.Manifest{}, true)

	f.store.Dispatch(ConfigsReloadedAction{TeamName: "sharks"})

	f.WaitUntil("teamName is set to sharks", func(state store.EngineState) bool {
		return state.TeamName == "sharks"
	})

	f.store.Dispatch(ConfigsReloadedAction{TeamName: "jets"})

	f.WaitUntil("teamName is set to jets", func(state store.EngineState) bool {
		return state.TeamName == "jets"
	})
}

func TestBuildLogAction(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	manifest := f.newManifest("alert-injester")
	f.Start([]model.Manifest{manifest}, true)

	f.store.Dispatch(BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    time.Now(),
	})

	f.store.Dispatch(BuildLogAction{
		LogEvent: store.NewLogEvent(manifest.Name, []byte(`a
bc
def
ghij`)),
	})

	f.WaitUntilManifestState("log appears", manifest.Name, func(ms store.ManifestState) bool {
		return ms.CurrentBuild.Log.Len() > 0
	})

	f.withState(func(s store.EngineState) {
		assert.Contains(t, s.Log.String(), `alert-injes…┊ a
alert-injes…┊ bc
alert-injes…┊ def
alert-injes…┊ ghij`)
	})

	err := f.Stop()
	assert.Nil(t, err)
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

type testOpter struct {
	calls []analytics.Opt
	mu    sync.Mutex
}

func (to *testOpter) SetOpt(opt analytics.Opt) error {
	to.mu.Lock()
	defer to.mu.Unlock()
	to.calls = append(to.calls, opt)
	return nil
}

func (to *testOpter) Calls() []analytics.Opt {
	to.mu.Lock()
	defer to.mu.Unlock()
	return append([]analytics.Opt{}, to.calls...)
}

func (to *testOpter) waitUntilCount(t *testing.T, expectedCount int) {
	timeout := time.After(time.Second)
	for {
		select {
		case <-time.After(5 * time.Millisecond):
			actualCount := len(to.Calls())
			if actualCount == expectedCount {
				return
			}
		case <-timeout:
			actualCount := len(to.Calls())
			t.Errorf("waiting for opt setting count to be %d. opt setting count is currently %d", expectedCount, actualCount)
			t.FailNow()
		}
	}
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
	kClient               *k8s.FakeK8sClient
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
	ghc                   *github.FakeClient
	opter                 *testOpter
	tiltVersionCheckDelay time.Duration

	// old value of k8sEventsFeatureFlag env var, for teardown
	// if nil, no reset needed.
	// TODO(maia): rm when we unflag this
	oldK8sEventsFeatureFlagVal *string

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
	to := &testOpter{}
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsWithOpterForTest(log, to)
	ctx, cancel := context.WithCancel(ctx)

	fSub := fixtureSub{ch: make(chan bool, 1000)}
	st := store.NewStore(UpperReducer, store.LogActionsFlag(false))
	st.AddSubscriber(ctx, fSub)

	plm := NewPodLogManager(k8s)
	bc := NewBuildController(b)

	err := os.Mkdir(f.JoinPath(".git"), os.FileMode(0777))
	if err != nil {
		t.Fatal(err)
	}

	fwm := NewWatchManager(watcher.newSub, timerMaker.maker())
	pfc := NewPortForwardController(k8s)
	ic := NewImageController(reaper)
	tas := NewTiltAnalyticsSubscriber(ta)
	ar := ProvideAnalyticsReporter(ta, st)

	fakeDcc := dockercompose.NewFakeDockerComposeClient(t, ctx)

	tfl := tiltfile.ProvideTiltfileLoader(ta, k8s, fakeDcc, "fake-context", feature.MainDefaults)
	cc := NewConfigsController(tfl, dockerClient)
	dcw := NewDockerComposeEventWatcher(fakeDcc)
	dclm := NewDockerComposeLogManager(fakeDcc)
	pm := NewProfilerManager()
	sCli := synclet.NewTestSyncletClient(dockerClient)
	sm := containerupdate.NewSyncletManagerForTests(k8s, sCli)
	hudsc := server.ProvideHeadsUpServerController(0, &server.HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{}, false)
	ghc := &github.FakeClient{}
	sc := &client.FakeSailClient{}
	ewm := NewEventWatchManager(k8s, clockwork.NewRealClock())

	ret := &testFixture{
		TempDirFixture:        f,
		ctx:                   ctx,
		cancel:                cancel,
		b:                     b,
		fsWatcher:             watcher,
		timerMaker:            &timerMaker,
		docker:                dockerClient,
		kClient:               k8s,
		hud:                   fakeHud,
		log:                   log,
		store:                 st,
		bc:                    bc,
		onchangeCh:            fSub.ch,
		fwm:                   fwm,
		cc:                    cc,
		dcc:                   fakeDcc,
		tfl:                   tfl,
		ghc:                   ghc,
		opter:                 to,
		tiltVersionCheckDelay: versionCheckInterval,
	}

	tiltVersionCheckTimerMaker := func(d time.Duration) <-chan time.Time {
		return time.After(ret.tiltVersionCheckDelay)
	}
	tvc := NewTiltVersionChecker(func() github.Client { return ghc }, tiltVersionCheckTimerMaker)

	subs := ProvideSubscribers(fakeHud, pw, sw, plm, pfc, fwm, bc, ic, cc, dcw, dclm, pm, sm, ar, hudsc, sc, tvc, tas, ewm)
	ret.upper = NewUpper(ctx, st, subs)

	go func() {
		fakeHud.Run(ctx, ret.upper.Dispatch, hud.DefaultRefreshInterval)
	}()

	return ret
}

func (f *testFixture) Start(manifests []model.Manifest, watchFiles bool, initOptions ...initOption) {
	f.startWithInitManifests(nil, manifests, watchFiles, initOptions...)
}

// Empty `initManifests` will run start ALL manifests
func (f *testFixture) startWithInitManifests(initManifests []model.ManifestName, manifests []model.Manifest, watchFiles bool, initOptions ...initOption) {
	ia := InitAction{
		Manifests:       manifests,
		WatchFiles:      watchFiles,
		TiltfilePath:    f.JoinPath("Tiltfile"),
		ExecuteTiltfile: true,
	}
	for _, o := range initOptions {
		ia = o(ia)
	}
	f.Init(ia)
}

type initOption func(ia InitAction) InitAction

func (f *testFixture) Init(action InitAction) {
	if action.TiltfilePath == "" {
		action.TiltfilePath = "/Tiltfile"
	}

	manifests := action.Manifests
	watchFiles := action.WatchFiles
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
		return len(st.ManifestTargets) == len(manifests) && st.WatchFiles == watchFiles
	})

	f.PollUntil("watches set up", func() bool {
		f.fwm.mu.Lock()
		defer f.fwm.mu.Unlock()
		return !watchFiles || len(f.fwm.targetWatches) == len(watchableTargetsForManifests(manifests))
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

func (f *testFixture) WaitUntilHUDResource(msg string, name model.ManifestName, isDone func(view.Resource) bool) {
	f.hud.WaitUntilResource(f.T(), f.ctx, msg, name, isDone)
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

func (f *testFixture) startPod(manifest model.Manifest) {
	pID := k8s.PodID("mypod")
	f.pod = f.testPod(pID.String(), manifest, "Running", time.Now())
	f.upper.store.Dispatch(PodChangeAction{f.pod})

	f.WaitUntilManifestState("pod appears", manifest.Name, func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == k8s.PodID(f.pod.Name)
	})
}

func (f *testFixture) podLog(manifestName model.ManifestName, s string) {
	f.upper.store.Dispatch(PodLogAction{
		PodID:    k8s.PodID(f.pod.Name),
		LogEvent: store.NewLogEvent(manifestName, []byte(s+"\n")),
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
		return ms.MostRecentPod().AllContainerRestarts() == int(restartCount)
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
	f.kClient.TearDown()
	close(f.fsWatcher.events)
	close(f.fsWatcher.errors)
	f.cancel()
}

func (f *testFixture) podEvent(pod *v1.Pod) {
	f.store.Dispatch(NewPodChangeAction(pod))
}

func (f *testFixture) newManifest(name string) model.Manifest {
	iTarget := NewSanchoLiveUpdateImageTarget(f)
	return manifestbuilder.New(f, model.ManifestName(name)).
		WithK8sYAML(SanchoYAML).
		WithImageTarget(iTarget).
		Build()
}

func (f *testFixture) newFastBuildManifest(name string, syncs []model.Sync) model.Manifest {
	ref := container.MustParseNamed(name)
	refSel := container.NewRefSelector(ref)
	iTarget := model.NewImageTarget(refSel).
		WithBuildDetails(model.FastBuild{
			BaseDockerfile: `from golang:1.10`,
			Syncs:          syncs,
		})
	return manifestbuilder.New(f, model.ManifestName(name)).
		WithK8sYAML(SanchoYAML).
		WithImageTarget(iTarget).
		Build()
}

func (f *testFixture) newManifestWithRef(name string, ref reference.Named) model.Manifest {
	refSel := container.NewRefSelector(ref)

	iTarget := NewSanchoLiveUpdateImageTarget(f)
	iTarget.ConfigurationRef = refSel
	iTarget.DeploymentRef = ref

	return manifestbuilder.New(f, model.ManifestName(name)).
		WithK8sYAML(SanchoYAML).
		WithImageTarget(iTarget).
		Build()
}

func (f *testFixture) newDCManifest(name string, DCYAMLRaw string, dockerfileContents string) model.Manifest {
	f.WriteFile("docker-compose.yml", DCYAMLRaw)
	return model.Manifest{
		Name: model.ManifestName(name),
	}.WithDeployTarget(model.DockerComposeTarget{
		ConfigPaths: []string{f.JoinPath("docker-compose.yml")},
		YAMLRaw:     []byte(DCYAMLRaw),
		DfRaw:       []byte(dockerfileContents),
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
		f.fsWatcher.events <- watch.NewFileEvent(filename)
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

	var dcConfig tiltfile.DcConfig
	err = yaml.Unmarshal(dcpc, &dcConfig)
	if err != nil {
		f.T().Fatal(err)
	}

	svc := dcConfig.Services["server"]
	svc.Build.Context = f.Path()
	dcConfig.Services["server"] = svc

	y, err := yaml.Marshal(dcConfig)
	if err != nil {
		f.T().Fatal(err)
	}
	f.dcc.ConfigOutput = string(y)

	f.dcc.ServicesOutput = "redis\nserver\n"

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

func (f *testFixture) hudResource(name model.ManifestName) view.Resource {
	res, ok := f.hud.LastView.Resource(name)
	if !ok {
		f.T().Fatalf("Resource not found: %s", name)
	}
	return res
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
		ref, _ := reference.WithTag(iTarget.DeploymentRef, "deadbeef")
		result := store.NewImageBuildResult(iTarget.ID(), ref)
		result.LiveUpdatedContainerIDs = []container.ID{id}
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

func assertContainsOnce(t *testing.T, s string, val string) {
	assert.Contains(t, s, val)
	assert.Equal(t, 1, strings.Count(s, val), "Expected string to appear only once")
}

func entityWithUID(t *testing.T, yaml string, uid string) k8s.K8sEntity {
	return entityWithUIDAndMaybeManifestLabel(t, yaml, uid, true)
}

func entityWithUIDAndMaybeManifestLabel(t *testing.T, yaml string, uid string, withManifestLabel bool) k8s.K8sEntity {
	es, err := k8s.ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatalf("error parsing yaml: %v", err)
	}

	if len(es) != 1 {
		t.Fatalf("expected exactly 1 k8s entity from yaml, got %d", len(es))
	}

	e := es[0]
	if withManifestLabel {
		e, err = k8s.InjectLabels(e, []model.LabelPair{{
			Key:   k8s.ManifestNameLabel,
			Value: e.Name(),
		}})
		if err != nil {
			t.Fatalf("error injecting manifest label: %v", err)
		}
	}

	k8s.SetUIDForTest(t, &e, uid)

	return e
}
