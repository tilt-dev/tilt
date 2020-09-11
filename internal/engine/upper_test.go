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
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/cloud"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/engine/dcwatch"
	"github.com/tilt-dev/tilt/internal/engine/dockerprune"
	"github.com/tilt-dev/tilt/internal/engine/exit"
	"github.com/tilt-dev/tilt/internal/engine/fswatch"
	"github.com/tilt-dev/tilt/internal/engine/k8srollout"
	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/engine/metrics"
	"github.com/tilt-dev/tilt/internal/engine/portforward"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/engine/telemetry"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/synclet"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/httptest"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/servicebuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/internal/tiltfile/config"
	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/internal/tracer"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

var originalWD string

type buildCompletionChannel chan bool

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

func (c buildAndDeployCall) local() model.LocalTarget {
	for _, spec := range c.specs {
		t, ok := spec.(model.LocalTarget)
		if ok {
			return t
		}
	}
	return model.LocalTarget{}
}

func (c buildAndDeployCall) dcState() store.BuildState {
	return c.state[c.dc().ID()]
}

func (c buildAndDeployCall) k8sState() store.BuildState {
	return c.state[c.k8s().ID()]
}

func (c buildAndDeployCall) oneImageState() store.BuildState {
	imageStates := make([]store.BuildState, 0)
	for k, v := range c.state {
		if k.Type == model.TargetTypeImage {
			imageStates = append(imageStates, v)
		}
	}

	if len(imageStates) != 1 {
		panic(fmt.Sprintf("More than one state: %v", c.state))
	}
	return imageStates[0]
}

type fakeBuildAndDeployer struct {
	t     *testing.T
	mu    sync.Mutex
	calls chan buildAndDeployCall

	completeBuildsManually bool
	buildCompletionChans   sync.Map // map[string]buildCompletionChannel; close channel at buildCompletionChans[k(targs)] to
	// complete the build started for targs (where k(targs) generates a unique string key for the set of targets)

	buildCount int

	// Set this to simulate a container update that returns the container IDs
	// it updated.
	nextLiveUpdateContainerIDs []container.ID

	// Inject the container ID of the container started by Docker Compose.
	// If not set, we will auto-generate an ID.
	nextDockerComposeContainerID    container.ID
	nextDockerComposeContainerState *dockertypes.ContainerState

	nextDeployedUID           types.UID
	nextPodTemplateSpecHashes []k8s.PodTemplateSpecHash

	// Set this to simulate a build with no results and an error.
	// Do not set this directly, use fixture.SetNextBuildError
	nextBuildError error

	// Set this to simulate a live-update compile error
	//
	// This is slightly different than a compile error, because the containers are
	// still running with the synced files. The build system returns a
	// result with the container ID.
	nextLiveUpdateCompileError error

	buildLogOutput map[model.TargetID]string

	resultsByID store.BuildResultSet
}

var _ BuildAndDeployer = &fakeBuildAndDeployer{}

func (b *fakeBuildAndDeployer) nextBuildResult(iTarget model.ImageTarget, deployTarget model.TargetSpec) store.BuildResult {
	tag := fmt.Sprintf("tilt-%d", b.buildCount)
	localRefTagged := container.MustWithTag(iTarget.Refs.LocalRef(), tag)
	clusterRefTagged := container.MustWithTag(iTarget.Refs.ClusterRef(), tag)

	var result store.BuildResult
	containerIDs := b.nextLiveUpdateContainerIDs
	if len(containerIDs) > 0 {
		result = store.NewLiveUpdateBuildResult(iTarget.ID(), containerIDs)
	} else {
		result = store.NewImageBuildResult(iTarget.ID(), localRefTagged, clusterRefTagged)
	}
	return result
}

func (b *fakeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, state store.BuildStateSet) (brs store.BuildResultSet, err error) {
	b.mu.Lock()
	b.buildCount++
	buildKey := stringifyTargetIDs(specs)
	b.registerBuild(buildKey)

	if !b.completeBuildsManually {
		// i.e. we should complete builds automatically: mark the build for completion now,
		// so we return immediately at the end of BuildAndDeploy.
		b.completeBuild(buildKey)
	}

	call := buildAndDeployCall{count: b.buildCount, specs: specs, state: state}
	if call.dc().Empty() && call.k8s().Empty() && call.local().Empty() {
		b.t.Fatalf("Invalid call: %+v", call)
	}

	ids := []model.TargetID{}
	for _, spec := range specs {
		id := spec.ID()
		ids = append(ids, id)
		output, ok := b.buildLogOutput[id]
		if ok {
			logger.Get(ctx).Infof("%s", output)
		}
	}

	defer func() {
		b.mu.Unlock()

		// block until we know we're supposed to resolve this build
		b.waitUntilBuildCompleted(ctx, buildKey)

		// don't update b.calls until the end, to ensure appropriate actions have been dispatched first
		select {
		case b.calls <- call:
		default:
			b.t.Error("writing to fakeBuildAndDeployer would block. either there's a bug or the buffer size needs to be increased")
		}

		logger.Get(ctx).Infof("fake built %s. error: %v", ids, err)
	}()

	err = b.nextBuildError
	b.nextBuildError = nil
	if err != nil {
		return nil, err
	}

	iTargets := model.ExtractImageTargets(specs)
	fakeImageExistsCheck := func(ctx context.Context, iTarget model.ImageTarget, namedTagged reference.NamedTagged) (bool, error) {
		return true, nil
	}
	queue, err := buildcontrol.NewImageTargetQueue(ctx, iTargets, state, fakeImageExistsCheck)
	if err != nil {
		return nil, err
	}

	err = queue.RunBuilds(func(target model.TargetSpec, depResults []store.BuildResult) (store.BuildResult, error) {
		iTarget := target.(model.ImageTarget)
		var deployTarget model.TargetSpec
		if !call.dc().Empty() {
			if isImageDeployedToDC(iTarget, call.dc()) {
				deployTarget = call.dc()
			}
		} else {
			if isImageDeployedToK8s(iTarget, call.k8s()) {
				deployTarget = call.k8s()
			}
		}

		return b.nextBuildResult(iTarget, deployTarget), nil
	})
	result := queue.NewResults()
	if err != nil {
		return result, err
	}

	if !call.dc().Empty() && len(b.nextLiveUpdateContainerIDs) == 0 {
		dcContainerID := container.ID(fmt.Sprintf("dc-%s", path.Base(call.dc().ID().Name.String())))
		if b.nextDockerComposeContainerID != "" {
			dcContainerID = b.nextDockerComposeContainerID
		}

		dcContainerState := b.nextDockerComposeContainerState
		result[call.dc().ID()] = store.NewDockerComposeDeployResult(
			call.dc().ID(), dcContainerID, dcContainerState)
	}

	if kTarg := call.k8s(); !kTarg.Empty() {
		uid := types.UID(uuid.New().String())
		if b.nextDeployedUID != "" {
			uid = b.nextDeployedUID
			b.nextDeployedUID = ""
		}
		uids := []types.UID{uid}

		templateSpecHashes := podTemplateSpecHashesForTarg(b.t, kTarg)
		if len(b.nextPodTemplateSpecHashes) != 0 {
			templateSpecHashes = b.nextPodTemplateSpecHashes
			b.nextPodTemplateSpecHashes = nil
		}
		result[call.k8s().ID()] = store.NewK8sDeployResult(call.k8s().ID(), uids, templateSpecHashes, nil)
	}

	err = b.nextLiveUpdateCompileError
	b.nextLiveUpdateCompileError = nil
	b.nextLiveUpdateContainerIDs = nil
	b.nextDockerComposeContainerID = ""

	for key, val := range result {
		b.resultsByID[key] = val
	}

	return result, err
}

func (b *fakeBuildAndDeployer) getOrCreateBuildCompletionChannel(key string) buildCompletionChannel {
	ch := make(buildCompletionChannel)
	val, _ := b.buildCompletionChans.LoadOrStore(key, ch)

	var ok bool
	ch, ok = val.(buildCompletionChannel)
	if !ok {
		panic(fmt.Sprintf("exected map value of type: buildCompletionChannel, got %T", val))
	}

	return ch
}

func (b *fakeBuildAndDeployer) registerBuild(key string) {
	b.getOrCreateBuildCompletionChannel(key)
}

func (b *fakeBuildAndDeployer) waitUntilBuildCompleted(ctx context.Context, key string) {
	ch := b.getOrCreateBuildCompletionChannel(key)

	// wait until channel for this build is closed, or context is canceled/finishes.
	select {
	case <-ch:
	case <-ctx.Done():
	}

	b.buildCompletionChans.Delete(key)
}

func newFakeBuildAndDeployer(t *testing.T) *fakeBuildAndDeployer {
	return &fakeBuildAndDeployer{
		t:              t,
		calls:          make(chan buildAndDeployCall, 20),
		buildLogOutput: make(map[model.TargetID]string),
		resultsByID:    store.BuildResultSet{},
	}
}

func (b *fakeBuildAndDeployer) completeBuild(key string) {
	ch := b.getOrCreateBuildCompletionChannel(key)
	close(ch)
}

func TestUpper_Up(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")

	f.setManifests([]model.Manifest{manifest})
	err := f.upper.Init(f.ctx, InitAction{
		EngineMode:   store.EngineModeApply,
		TiltfilePath: f.JoinPath("Tiltfile"),
		StartTime:    f.Now(),
	})
	close(f.b.calls)
	require.NoError(t, err)
	var started []model.TargetID
	for call := range f.b.calls {
		started = append(started, call.k8s().ID())
	}
	require.Equal(t, []model.TargetID{manifest.K8sTarget().ID()}, started)

	state := f.upper.store.RLockState()
	defer f.upper.store.RUnlockState()

	buildRecord := state.ManifestTargets[manifest.Name].Status().LastBuild()
	lines := strings.Split(state.LogStore.SpanLog(buildRecord.SpanID), "\n")
	assertLineMatches(t, lines, regexp.MustCompile("fake built .*foobar"))
}

func TestUpper_UpK8sEntityOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	postgresEntities, err := k8s.ParseYAMLFromString(testyaml.PostgresYAML)
	require.NoError(t, err)
	yaml, err := k8s.SerializeSpecYAML(postgresEntities[:3]) // only take entities that don't belong to a workload
	require.NoError(t, err)
	f.WriteFile("Tiltfile", `k8s_yaml('postgres.yaml')`)
	f.WriteFile("postgres.yaml", yaml)

	err = f.upper.Init(f.ctx, InitAction{
		EngineMode:   store.EngineModeApply,
		TiltfilePath: f.JoinPath("Tiltfile"),
		StartTime:    f.Now(),
	})
	require.NoError(t, err)

	call := f.nextCall()
	entities, err := k8s.ParseYAMLFromString(call.k8s().YAML)
	require.NoError(t, err)
	expectedKindOrder := []string{"PersistentVolume", "PersistentVolumeClaim", "ConfigMap"}
	actualKindOrder := make([]string, len(entities))
	for i, e := range entities {
		actualKindOrder[i] = e.GVK().Kind
	}
	assert.Equal(t, expectedKindOrder, actualKindOrder,
		"YAML on the manifest should be in sorted order")

	f.assertAllBuildsConsumed()
}

func TestUpper_WatchFalseNoManifestsExplicitlyNamed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	err := f.upper.Init(f.ctx, InitAction{
		EngineMode:   store.EngineModeApply,
		TiltfilePath: f.JoinPath("Tiltfile"),
		UserArgs:     nil, // equivalent to `tilt up --watch=false` (i.e. not specifying any manifest names)
		StartTime:    f.Now(),
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
	f.Start([]model.Manifest{manifest})

	f.fsWatcher.Errors <- context.Canceled

	err := <-f.upperInitResult
	if assert.NotNil(t, err) {
		assert.Equal(t, "context canceled", err.Error())
	}
}

func TestUpper_UpWatchFileChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	f.timerMaker.MaxTimerLock.Lock()
	call := f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	fileRelPath := "fdas"
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(fileRelPath))

	call = f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-1", call.oneImageState().LastLocalImageAsString())
	fileAbsPath := f.JoinPath(fileRelPath)
	assert.Equal(t, []string{fileAbsPath}, call.oneImageState().FilesChanged())

	f.withManifestState("foobar", func(ms store.ManifestState) {
		assert.True(t, ms.LastBuild().Reason.Has(model.BuildReasonFlagChangedFiles))
		assert.True(t, ms.LastBuild().HasBuildType(model.BuildTypeImage))
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChanges(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	f.timerMaker.MaxTimerLock.Lock()
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	f.timerMaker.RestTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(fileRelPath))
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.RestTimerLock.Unlock()

	call = f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())

	var fileAbsPaths []string
	for _, fileRelPath := range fileRelPaths {
		fileAbsPaths = append(fileAbsPaths, f.JoinPath(fileRelPath))
	}
	assert.Equal(t, fileAbsPaths, call.oneImageState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestUpper_UpWatchCoalescedFileChangesHitMaxTimeout(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	f.timerMaker.MaxTimerLock.Lock()
	f.timerMaker.RestTimerLock.Lock()
	fileRelPaths := []string{"fdas", "giueheh"}
	for _, fileRelPath := range fileRelPaths {
		f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(fileRelPath))
	}
	time.Sleep(time.Millisecond)
	f.timerMaker.MaxTimerLock.Unlock()

	call = f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())

	var fileAbsPaths []string
	for _, fileRelPath := range fileRelPaths {
		fileAbsPaths = append(fileAbsPaths, f.JoinPath(fileRelPath))
	}
	assert.Equal(t, fileAbsPaths, call.oneImageState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.SetNextBuildError(errors.New("Build failed"))

	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("a.go"))

	call = f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneImageState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestFirstBuildCancelsWhileWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.SetNextBuildError(context.Canceled)

	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestFirstBuildFailsWhileNotWatching(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	buildFailedToken := errors.New("doesn't compile")
	f.SetNextBuildError(buildFailedToken)

	f.setManifests([]model.Manifest{manifest})
	f.Init(InitAction{
		EngineMode:   store.EngineModeApply,
		TiltfilePath: f.JoinPath("Tiltfile"),
		TerminalMode: store.TerminalModeHUD,
		StartTime:    f.Now(),
	})

	f.WaitUntilManifestState("build has failed", manifest.ManifestName(), func(st store.ManifestState) bool {
		return st.LastBuild().Error != nil
	})

	select {
	case err := <-f.upperInitResult:
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "doesn't compile")
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for exit action")
	}

	f.withState(func(es store.EngineState) {
		assert.True(t, es.ExitSignal)
	})
}

func TestRebuildWithChangedFiles(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneImageState().IsEmpty())
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	// Simulate a change to a.go that makes the build fail.
	f.SetNextBuildError(errors.New("build failed"))
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("a.go"))

	call = f.nextCallComplete("failed build from a.go change")
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-1", call.oneImageState().LastLocalImageAsString())
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneImageState().FilesChanged())

	// Simulate a change to b.go
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("b.go"))

	// The next build should only treat b.go as changed.
	call = f.nextCallComplete("build on last successful result")
	assert.Equal(t, []string{f.JoinPath("b.go")}, call.oneImageState().FilesChanged())
	assert.Equal(t, "gcr.io/some-project-162817/sancho:tilt-1",
		call.oneImageState().LastLocalImageAsString())

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestThreeBuilds(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneImageState().IsEmpty())
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("a.go"))

	call = f.nextCallComplete("second build")
	assert.Equal(t, []string{f.JoinPath("a.go")}, call.oneImageState().FilesChanged())
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	// Simulate a change to b.go
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("b.go"))

	call = f.nextCallComplete("third build")
	assert.Equal(t, []string{f.JoinPath("b.go")}, call.oneImageState().FilesChanged())
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	// Simulate a change to .#a.go that's a broken symlink.
	realPath := filepath.Join(f.Path(), "a.go")
	tmpPath := filepath.Join(f.Path(), ".#a.go")
	_ = os.Symlink(realPath, tmpPath)

	f.fsWatcher.Events <- watch.NewFileEvent(tmpPath)

	f.assertNoCall()

	f.TouchFiles([]string{realPath})
	f.fsWatcher.Events <- watch.NewFileEvent(realPath)

	call = f.nextCall()
	assert.Equal(t, []string{realPath}, call.oneImageState().FilesChanged())

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
	// (i.e. check that we called BuildAndDeploy with no pre-existing state)
	assert.False(t, call.oneImageState().HasLastResult())

	var manifest model.Manifest
	f.withState(func(es store.EngineState) {
		manifest = es.Manifests()[0]
	})
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("random_file.go"))

	// third call: new manifest should persist
	call = f.nextCall("persist new manifest")
	assert.Equal(t, "FROM iron/go:dev", call.firstImgTarg().DockerBuildInfo().Dockerfile)

	// Unchanged manifest --> we do NOT clear the build state
	assert.True(t, call.oneImageState().HasLastResult())

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
	assert.ElementsMatch(t, []string{}, call.oneImageState().FilesChanged())

	// Since the manifest changed, we cleared the previous build state to force an image build
	assert.False(t, call.oneImageState().HasLastResult())

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
	assert.ElementsMatch(t, []string{}, call.oneImageState().FilesChanged())

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
	f.fsWatcher.Events <- watch.NewFileEvent(changed)

	call = f.nextCall("build from file change")
	assert.Equal(t, "snack", string(call.k8s().Name))
	assert.ElementsMatch(t, []string{
		f.JoinPath("src/main.go"),
	}, call.oneImageState().FilesChanged())
	assert.True(t, call.oneImageState().HasLastResult(), "Unchanged manifest --> we do NOT clear the build state")

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
		assert.Equal(t, "", buildcontrol.NextManifestNameToBuild(state).String())
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
	assert.False(t, call.oneImageState().HasLastResult(),
		"expected this call to have NO image (since we should have cleared it to force an image build)")

	f.WaitUntil("tiltfile error cleared", func(state store.EngineState) bool {
		return state.LastTiltfileError() == nil
	})

	f.withManifestTarget("snack", func(mt store.ManifestTarget) {
		expectedCmd := model.ToUnixCmd("changed")
		assert.Equal(t, expectedCmd, mt.Manifest.ImageTargetAt(0).LiveUpdateInfo().RunSteps()[0].Cmd,
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
		return mt.Manifest.TriggerMode == model.TriggerModeManualAfterInitial
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
	f.WaitUntilManifest("manifest has triggerMode = manual_after_initial", "snack", func(mt store.ManifestTarget) bool {
		imageTargetID = mt.Manifest.ImageTargetAt(0).ID() // grab for later
		return mt.Manifest.TriggerMode == model.TriggerModeManualAfterInitial
	})

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("src/main.go"))
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(imageTargetID).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	// Update Tiltfile to change the trigger mode of the manifest
	triggerAutoTiltfile := fmt.Sprintf(baseTiltfile, "TRIGGER_MODE_AUTO")
	f.WriteConfigFiles("Tiltfile", triggerAutoTiltfile)

	call := f.nextCall("manifest updated b/c it's now TriggerModeAuto")
	assert.True(t, call.oneImageState().HasLastResult(),
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

func TestConfigChange_ManifestIncludingInitialBuildsIfTriggerModeChangedToManualAfterInitial(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	foo := f.newManifest("foo").WithTriggerMode(model.TriggerModeManualIncludingInitial)
	bar := f.newManifest("bar")

	f.Start([]model.Manifest{foo, bar})

	// foo should be skipped, and just bar built
	call := f.nextCallComplete("initial build")
	require.Equal(t, bar.ImageTargetAt(0), call.firstImgTarg())

	// since foo is "Manual", it should not be built on startup
	// make sure there's nothing waiting to build
	f.withState(func(state store.EngineState) {
		n := buildcontrol.NextManifestNameToBuild(state)
		require.Equal(t, model.ManifestName(""), n)
	})

	// change the trigger mode
	foo = foo.WithTriggerMode(model.TriggerModeManualAfterInitial)
	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime: f.Now(),
		Manifests:  []model.Manifest{foo, bar},
	})

	// now that it is a trigger mode that should build on startup, a build should kick off
	// even though we didn't trigger anything
	call = f.nextCallComplete("second build")
	require.Equal(t, foo.ImageTargetAt(0), call.firstImgTarg())

	err := f.Stop()
	assert.Nil(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigChange_FilenamesLoggedInManifestBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
k8s_yaml('snack.yaml')
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src')`)
	f.WriteFile("src/Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	f.WaitUntilManifestState("snack loaded", "snack", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) == 1
	})

	// make a config file change to kick off a new build
	f.WriteFile("Tiltfile", `
k8s_yaml('snack.yaml')
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', ignore='Dockerfile')`)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("Tiltfile"))

	f.WaitUntilManifestState("snack reloaded", "snack", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) == 2
	})

	f.withState(func(es store.EngineState) {
		expected := fmt.Sprintf("1 File Changed: [%s]", f.JoinPath("Tiltfile"))
		require.Contains(t, es.LogStore.ManifestLog("snack"), expected)
	})

	err := f.Stop()
	assert.Nil(t, err)
}

func TestConfigChange_LocalResourceChange(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `print('tiltfile 1')
local_resource('local', 'echo one fish two fish', deps='foo.bar')`)

	f.loadAndStart()

	// First call: with the old manifests
	call := f.nextCall("initial call")
	assert.Equal(t, "local", string(call.local().Name))
	assert.Equal(t, "echo one fish two fish", call.local().UpdateCmd.String())

	// Change the definition of the resource -- this changes the manifest which should trigger an updated
	f.WriteConfigFiles("Tiltfile", `print('tiltfile 2')
local_resource('local', 'echo red fish blue fish', deps='foo.bar')`)
	call = f.nextCall("rebuild from config change")
	assert.Equal(t, "echo red fish blue fish", call.local().UpdateCmd.String())

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

	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete("first build")
	assert.True(t, call.oneImageState().IsEmpty())

	// Simulate a change to main.go
	mainPath := filepath.Join(f.Path(), "main.go")
	f.fsWatcher.Events <- watch.NewFileEvent(mainPath)

	// Check that this triggered a rebuild.
	call = f.nextCallComplete("rebuild triggered")
	assert.Equal(t, []string{mainPath}, call.oneImageState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestHudUpdated(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("foobar")

	f.Start([]model.Manifest{manifest})
	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	f.WaitUntilHUD("hud update", func(v view.View) bool {
		return len(v.Resources) == 2
	})

	err := f.Stop()
	assert.Equal(t, nil, err)

	assert.Equal(t, 2, len(f.fakeHud().LastView.Resources))
	assert.Equal(t, store.TiltfileManifestName, f.fakeHud().LastView.Resources[0].Name)
	rv := f.fakeHud().LastView.Resources[1]
	assert.Equal(t, manifest.Name, model.ManifestName(rv.Name))
	f.assertAllBuildsConsumed()
}

func TestDisabledHudUpdated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): Investigate")
	}
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("foobar")
	opt := func(ia InitAction) InitAction {
		ia.TerminalMode = store.TerminalModeStream
		return ia
	}

	f.Start([]model.Manifest{manifest}, opt)
	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	// Make sure we're done logging stuff, then grab # processed bytes
	f.WaitUntil("foobar logs appear", func(es store.EngineState) bool {
		return strings.Contains(f.log.String(), "Initial Build • foobar")
	})

	assert.True(t, f.ts.ProcessedLogs > 0)
	oldCheckpoint := f.ts.ProcessedLogs

	// Log something new, make sure it's reflected
	msg := []byte("hello world!\n")
	f.store.Dispatch(store.NewGlobalLogAction(logger.InfoLvl, msg))

	f.WaitUntil("hello world logs appear", func(es store.EngineState) bool {
		return strings.Contains(f.log.String(), "hello world!")
	})

	assert.True(t, f.ts.ProcessedLogs > oldCheckpoint)

	err := f.Stop()
	assert.Equal(t, nil, err)

	f.assertAllBuildsConsumed()

}

func TestPodEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	pod := podbuilder.New(f.T(), manifest).
		WithPhase("CrashLoopBackOff").
		Build()
	f.podEvent(pod, manifest.Name)

	f.WaitUntilHUDResource("hud update", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodName == pod.Name
	})

	rv := f.hudResource("foobar")
	assert.Equal(t, pod.Name, rv.K8sInfo().PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.K8sInfo().PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodResetRestartsAction(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	pb := podbuilder.New(f.T(), manifest)
	f.podEvent(pb.Build(), manifest.Name)

	pb = pb.
		WithPhase("CrashLoopBackOff").
		WithRestartCount(1)
	f.podEvent(pb.Build(), manifest.Name)

	f.WaitUntilManifestState("restart seen", "fe", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().VisibleContainerRestarts() == 1
	})

	f.store.Dispatch(store.NewPodResetRestartsAction(
		k8s.PodID(pb.Build().Name), "fe", 1))

	f.WaitUntilManifestState("restart cleared", "fe", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().VisibleContainerRestarts() == 0
	})

	assert.NoError(t, f.Stop())
}

func TestPodEventOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")

	past := f.Now().Add(-time.Minute)
	now := f.Now()
	uidPast := types.UID("deployment-uid-past")
	uidNow := types.UID("deployment-uid-now")
	pb := podbuilder.New(f.T(), manifest)
	pbA := pb.WithPodID("pod-a")
	pbB := pb.WithPodID("pod-b")
	pbC := pb.WithPodID("pod-c")
	podAPast := pbA.WithCreationTime(past).WithDeploymentUID(uidPast)
	podBPast := pbB.WithCreationTime(past).WithDeploymentUID(uidPast)
	podANow := pbA.WithCreationTime(now).WithDeploymentUID(uidNow)
	podBNow := pbB.WithCreationTime(now).WithDeploymentUID(uidNow)
	podCNow := pbC.WithCreationTime(now).WithDeploymentUID(uidNow)
	podCNowDeleting := podCNow.WithDeletionTime(now)

	// Test the pod events coming in in different orders,
	// and the manifest ends up with podANow and podBNow
	podOrders := [][]podbuilder.PodBuilder{
		{podAPast, podBPast, podANow, podBNow},
		{podAPast, podANow, podBPast, podBNow},
		{podANow, podAPast, podBNow, podBPast},
		{podAPast, podBPast, podANow, podCNow, podCNowDeleting, podBNow},
	}

	for i, order := range podOrders {

		t.Run(fmt.Sprintf("TestPodOrder%d", i), func(t *testing.T) {
			f := newTestFixture(t)
			defer f.TearDown()
			f.b.nextDeployedUID = uidNow
			f.Start([]model.Manifest{manifest})

			call := f.nextCall()
			assert.True(t, call.oneImageState().IsEmpty())
			f.WaitUntilManifestState("uid deployed", "fe", func(ms store.ManifestState) bool {
				return ms.K8sRuntimeState().DeployedUIDSet.Contains(uidNow)
			})

			for _, pb := range order {
				f.store.Dispatch(k8swatch.NewPodChangeAction(pb.Build(), manifest.Name, pb.DeploymentUID()))
			}

			f.upper.store.Dispatch(
				store.NewLogAction("fe", runtimelog.SpanIDForPod(podBNow.PodID()), logger.InfoLvl, nil, []byte("pod b log\n")))

			f.WaitUntil("pod log seen", func(state store.EngineState) bool {
				ms, _ := state.ManifestState("fe")
				spanID := ms.MostRecentPod().SpanID
				return spanID != "" && strings.Contains(state.LogStore.SpanLog(spanID), "pod b log")
			})

			f.withManifestState("fe", func(ms store.ManifestState) {
				runtime := ms.K8sRuntimeState()
				if assert.Equal(t, 2, runtime.PodLen()) {
					assert.Equal(t, now.String(), runtime.Pods["pod-a"].StartedAt.String())
					assert.Equal(t, now.String(), runtime.Pods["pod-b"].StartedAt.String())
					assert.Equal(t, uidNow, runtime.PodAncestorUID)
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
	f.Start([]model.Manifest{manifest})

	var ref reference.NamedTagged
	f.WaitUntilManifestState("image appears", "foobar", func(ms store.ManifestState) bool {
		result := ms.BuildStatus(manifest.ImageTargetAt(0).ID()).LastResult
		ref = store.ClusterImageRefFromBuildResult(result)
		return ref != nil
	})

	pod := podbuilder.New(f.T(), manifest).Build()
	pod.Status = k8s.FakePodStatus(ref, "Running")
	pod.Status.ContainerStatuses[0].ContainerID = ""
	pod.Spec = k8s.FakePodSpec(ref)
	f.podEvent(pod, manifest.Name)

	podState := store.Pod{}
	f.WaitUntilManifestState("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.MostRecentPod()
		return podState.PodID.String() == pod.Name && len(podState.Containers) > 0
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
	}.WithDeployTarget(k8s.MustTarget("foobar", SanchoYAML))
	ref := container.MustParseNamedTagged("dockerhub/we-didnt-build-this:foo")
	f.Start([]model.Manifest{manifest})

	f.WaitUntilManifestState("first build complete", "foobar", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) > 0
	})

	pod := podbuilder.New(f.T(), manifest).Build()
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

	f.podEvent(pod, manifest.Name)

	podState := store.Pod{}
	f.WaitUntilManifestState("container status", "foobar", func(ms store.ManifestState) bool {
		podState = ms.MostRecentPod()
		return podState.PodID.String() == pod.Name && len(podState.Containers) > 0
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

	ptsh := k8s.PodTemplateSpecHash("hash")
	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())
	f.Start([]model.Manifest{manifest})
	f.registerDeployedPodTemplateSpecHashToManifest(name, ptsh)

	// Start and end a fake build to set manifestState.ExpectedContainerId
	f.store.Dispatch(fswatch.NewTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))

	f.WaitUntil("builds ready & changed file recorded", func(st store.EngineState) bool {
		ms, _ := st.ManifestState(manifest.Name)
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name && ms.HasPendingFileChanges()
	})
	spanID0 := SpanIDForBuildLog(0)
	f.store.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    f.Now(),
		SpanID:       spanID0,
	})

	f.store.Dispatch(buildcontrol.NewBuildCompleteAction(name,
		spanID0,
		liveUpdateResultSet(manifest, "theOriginalContainer"), nil))

	f.WaitUntil("nothing waiting for build", func(st store.EngineState) bool {
		return st.CompletedBuildCount == 1 && buildcontrol.NextManifestNameToBuild(st) == ""
	})

	f.podEvent(podbuilder.New(t, manifest).
		WithPodID("mypod").
		WithTemplateSpecHash(ptsh).
		WithContainerID("myfunnycontainerid").
		Build(), manifest.Name)

	f.WaitUntilManifestState("NeedsRebuildFromCrash set to True", "foobar", func(ms store.ManifestState) bool {
		return ms.NeedsRebuildFromCrash
	})

	f.WaitUntil("manifest queued for build b/c it's crashing", func(st store.EngineState) bool {
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name
	})
}

func TestPodUnexpectedContainerStartsImageBuildOutOfOrderEvents(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())
	ptsh := k8s.PodTemplateSpecHash("abc123hash")

	f.Start([]model.Manifest{manifest})

	// (recognize incoming pods with this ptsh as belonging to this manifest)
	f.registerDeployedPodTemplateSpecHashToManifest(name, ptsh)

	// Start a fake build
	f.store.Dispatch(fswatch.NewTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))
	f.WaitUntil("builds ready & changed file recorded", func(st store.EngineState) bool {
		ms, _ := st.ManifestState(manifest.Name)
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name && ms.HasPendingFileChanges()
	})
	spanID0 := SpanIDForBuildLog(0)
	f.store.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    f.Now(),
		SpanID:       spanID0,
	})

	// Simulate k8s restarting the container due to a crash.
	f.podEvent(podbuilder.New(t, manifest).WithContainerID("myfunnycontainerid").
		WithTemplateSpecHash(ptsh).Build(), manifest.Name)

	// ...and finish the build. Even though this action comes in AFTER the pod
	// event w/ unexpected container,  we should still be able to detect the mismatch.
	f.store.Dispatch(buildcontrol.NewBuildCompleteAction(name, spanID0,
		liveUpdateResultSet(manifest, "theOriginalContainer"), nil))

	f.WaitUntilManifestState("NeedsRebuildFromCrash set to True", "foobar", func(ms store.ManifestState) bool {
		return ms.NeedsRebuildFromCrash
	})
	f.WaitUntil("manifest queued for build b/c it's crashing", func(st store.EngineState) bool {
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name
	})
}

func TestPodUnexpectedContainerAfterSuccessfulUpdate(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())
	ptsh := k8s.PodTemplateSpecHash("abc123hash")

	f.Start([]model.Manifest{manifest})

	// Start and end a normal build
	f.store.Dispatch(fswatch.NewTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))
	f.WaitUntil("builds ready & changed file recorded", func(st store.EngineState) bool {
		ms, _ := st.ManifestState(manifest.Name)
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name && ms.HasPendingFileChanges()
	})

	spanID0 := SpanIDForBuildLog(0)
	f.store.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    f.Now(),
		SpanID:       spanID0,
	})
	podStartTime := f.Now()
	ancestorUID := types.UID("fake-uid")
	pb := podbuilder.New(t, manifest).
		WithPodID("mypod").
		WithContainerID("normal-container-id").
		WithCreationTime(podStartTime).
		WithDeploymentUID(ancestorUID).
		WithTemplateSpecHash(ptsh)
	f.store.Dispatch(buildcontrol.NewBuildCompleteAction(name,
		spanID0,
		deployResultSet(manifest, ancestorUID, []k8s.PodTemplateSpecHash{ptsh}), nil))

	f.store.Dispatch(k8swatch.NewPodChangeAction(pb.Build(), manifest.Name, ancestorUID))
	f.WaitUntil("nothing waiting for build", func(st store.EngineState) bool {
		return st.CompletedBuildCount == 1 && buildcontrol.NextManifestNameToBuild(st) == ""
	})

	// Start another fake build
	f.store.Dispatch(fswatch.NewTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a"))
	f.WaitUntil("waiting for builds to be ready", func(st store.EngineState) bool {
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name
	})

	spanID1 := SpanIDForBuildLog(1)
	f.store.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    f.Now(),
		SpanID:       spanID1,
	})

	// Simulate a pod crash, then a build completion
	pb = pb.WithContainerID("funny-container-id")
	f.store.Dispatch(k8swatch.NewPodChangeAction(pb.Build(), manifest.Name, ancestorUID))

	f.store.Dispatch(buildcontrol.NewBuildCompleteAction(name,
		spanID1,
		liveUpdateResultSet(manifest, "normal-container-id"), nil))

	f.WaitUntilManifestState("NeedsRebuildFromCrash set to True", "foobar", func(ms store.ManifestState) bool {
		return ms.NeedsRebuildFromCrash
	})

	f.WaitUntil("manifest queued for build b/c it's crashing", func(st store.EngineState) bool {
		return buildcontrol.NextManifestNameToBuild(st) == manifest.Name
	})
}

func TestPodEventUpdateByTimestamp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	firstCreationTime := f.Now()
	pod := podbuilder.New(f.T(), manifest).
		WithCreationTime(firstCreationTime).
		WithPhase("CrashLoopBackOff").
		Build()
	f.podEvent(pod, manifest.Name)
	f.WaitUntilHUDResource("hud update crash", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "CrashLoopBackOff"
	})

	newPod := podbuilder.New(f.T(), manifest).
		WithPodID("my-new-pod").
		WithCreationTime(firstCreationTime.Add(time.Minute * 2)).
		Build()
	f.podEvent(newPod, manifest.Name)
	f.WaitUntilHUDResource("hud update running", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "Running"
	})

	rv := f.hudResource("foobar")
	assert.Equal(t, newPod.Name, rv.K8sInfo().PodName)
	assert.Equal(t, "Running", rv.K8sInfo().PodStatus)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()
}

func TestPodEventDeleted(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mn := model.ManifestName("foobar")
	manifest := f.newManifest(mn.String())
	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete()
	assert.True(t, call.oneImageState().IsEmpty())

	creationTime := f.Now()
	pb := podbuilder.New(f.T(), manifest).
		WithCreationTime(creationTime)
	f.podEvent(pb.Build(), manifest.Name)

	f.WaitUntilManifestState("pod crashes", mn, func(state store.ManifestState) bool {
		return state.K8sRuntimeState().ContainsID(k8s.PodID(pb.PodID()))
	})

	f.podEvent(pb.WithDeletionTime(creationTime.Add(time.Minute)).Build(), manifest.Name)

	f.WaitUntilManifestState("podset is empty", mn, func(state store.ManifestState) bool {
		return state.K8sRuntimeState().PodLen() == 0
	})

	err := f.Stop()
	if err != nil {
		t.Fatal(err)
	}

	f.assertAllBuildsConsumed()
}

func TestPodEventUpdateByPodName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("foobar")
	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete()
	assert.True(t, call.oneImageState().IsEmpty())

	creationTime := f.Now()
	pb := podbuilder.New(f.T(), manifest).
		WithCreationTime(creationTime).
		WithPhase("CrashLoopBackOff")
	f.podEvent(pb.Build(), manifest.Name)

	f.WaitUntilHUDResource("pod crashes", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "CrashLoopBackOff"
	})

	f.podEvent(pb.WithPhase("Running").Build(), manifest.Name)

	f.WaitUntilHUDResource("pod comes back", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "Running"
	})

	rv := f.hudResource("foobar")
	assert.Equal(t, pb.Build().Name, rv.K8sInfo().PodName)
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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.True(t, call.oneImageState().IsEmpty())

	creationTime := f.Now()
	pod := podbuilder.New(f.T(), manifest).
		WithPodID("my-new-pod").
		WithPhase("CrashLoopBackOff").
		WithCreationTime(creationTime).
		Build()
	f.podEvent(pod, manifest.Name)
	f.WaitUntilHUDResource("hud update", "foobar", func(res view.Resource) bool {
		return res.K8sInfo().PodStatus == "CrashLoopBackOff"
	})

	oldPod := podbuilder.New(f.T(), manifest).
		WithCreationTime(creationTime.Add(time.Minute * -1)).
		Build()
	f.podEvent(oldPod, manifest.Name)
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, f.Stop())
	f.assertAllBuildsConsumed()

	rv := f.hudResource("foobar")
	assert.Equal(t, pod.Name, rv.K8sInfo().PodName)
	assert.Equal(t, "CrashLoopBackOff", rv.K8sInfo().PodStatus)
}

func TestPodContainerStatus(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	_ = f.nextCall()

	var ref reference.NamedTagged
	f.WaitUntilManifestState("image appears", "fe", func(ms store.ManifestState) bool {
		result := ms.BuildStatus(manifest.ImageTargetAt(0).ID()).LastResult
		ref = store.ClusterImageRefFromBuildResult(result)
		return ref != nil
	})

	startedAt := f.Now()
	pb := podbuilder.New(f.T(), manifest).
		WithCreationTime(startedAt)
	pod := pb.Build()
	f.podEvent(pod, manifest.Name)
	f.WaitUntilManifestState("pod appears", "fe", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID.String() == pod.Name
	})

	pod = pb.Build()
	pod.Spec = k8s.FakePodSpec(ref)
	pod.Status = k8s.FakePodStatus(ref, "Running")
	f.podEvent(pod, manifest.Name)

	f.WaitUntilManifestState("container is ready", "fe", func(ms store.ManifestState) bool {
		ports := ms.MostRecentPod().AllContainerPorts()
		return len(ports) == 1 && ports[0] == 8080
	})

	err := f.Stop()
	assert.NoError(t, err)

	f.assertAllBuildsConsumed()
}

func TestPodAddedToStateOrNotByTemplateHash(t *testing.T) {
	deployedHash := k8s.PodTemplateSpecHash("some-hash-abc")
	nonMatchingHash := k8s.PodTemplateSpecHash("danger-will-robinson")
	ancestorUID := types.UID("some-ancestor")
	podID := k8s.PodID("special-pod")
	otherPodID := k8s.PodID("other-pod")
	mName := model.ManifestName("foobar")

	tests := []struct {
		name                      string
		ptshMatch                 bool // does the pod template spec hash of incoming pod event match the PTSH we most recently deployed?
		ancestorSeen              bool // does the k8s runtime state already know about the ancestor UID of our pod?
		podSeen                   bool // instantiate runtime state with an old version of the pod we're getting events for
		haveAdditionalPod         bool // runtime state knows about another pod besides the one we're getting events for
		expectUpdatePodOnManifest bool // expect pod event to cause an update to manifestState.Pods
	}{
		{name: "new ancestor + ptsh OK",
			ptshMatch:                 true,
			ancestorSeen:              false,
			podSeen:                   false,
			haveAdditionalPod:         false,
			expectUpdatePodOnManifest: true,
		}, {name: "new ancestor + ptsh not OK",
			ptshMatch:                 false,
			ancestorSeen:              false,
			podSeen:                   false,
			haveAdditionalPod:         false,
			expectUpdatePodOnManifest: false,
		}, {name: "existing ancestor/new pod + ptsh OK",
			ptshMatch:                 true,
			ancestorSeen:              true,
			podSeen:                   false,
			haveAdditionalPod:         false,
			expectUpdatePodOnManifest: true,
		}, {name: "existing ancestor/new pod + ptsh OK + additional pod",
			ptshMatch:                 true,
			ancestorSeen:              true,
			podSeen:                   false,
			haveAdditionalPod:         true,
			expectUpdatePodOnManifest: true,
		}, {name: "existing ancestor/new pod + ptsh not OK",
			ptshMatch:                 false,
			ancestorSeen:              true,
			podSeen:                   false,
			haveAdditionalPod:         false,
			expectUpdatePodOnManifest: false,
		}, {name: "existing ancestor/new pod + ptsh not OK + additional pod",
			ptshMatch:                 false,
			ancestorSeen:              true,
			podSeen:                   false,
			haveAdditionalPod:         true,
			expectUpdatePodOnManifest: false,
		}, {name: "existing pod + ptsh OK",
			ptshMatch:                 true,
			ancestorSeen:              true,
			podSeen:                   true,
			haveAdditionalPod:         false,
			expectUpdatePodOnManifest: true,
		}, {name: "existing pod + ptsh OK + additional pod",
			ptshMatch:                 true,
			ancestorSeen:              true,
			podSeen:                   true,
			haveAdditionalPod:         true,
			expectUpdatePodOnManifest: true,
		}, {name: "existing pod + ptsh not OK",
			ptshMatch:                 false,
			ancestorSeen:              true,
			podSeen:                   true,
			haveAdditionalPod:         false,
			expectUpdatePodOnManifest: true,
		},
		{name: "existing pod + ptsh not OK + additional pod",
			ptshMatch:                 false,
			ancestorSeen:              true,
			podSeen:                   true,
			haveAdditionalPod:         true,
			expectUpdatePodOnManifest: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newTestFixture(t)
			defer f.TearDown()
			manifest := f.newManifest(mName.String())

			f.b.nextDeployedUID = ancestorUID
			f.b.nextPodTemplateSpecHashes = []k8s.PodTemplateSpecHash{deployedHash}
			f.Start([]model.Manifest{manifest})

			_ = f.nextCallComplete()

			// set up runtime state
			st := f.store.LockMutableStateForTesting()
			ms, ok := st.ManifestState(model.ManifestName(mName))
			require.True(t, ok, "couldn't find manifest state for %s", mName)

			runtime := ms.K8sRuntimeState()
			runtime.Pods = make(map[k8s.PodID]*store.Pod)
			if test.ancestorSeen {
				runtime.PodAncestorUID = ancestorUID
			}

			if test.podSeen {
				runtime.Pods[podID] = &store.Pod{
					PodID:  podID,
					Status: "Running",
				}
			}
			if test.haveAdditionalPod {
				runtime.Pods[otherPodID] = &store.Pod{
					PodID:  otherPodID,
					Status: "Running",
				}
			}
			ms.RuntimeState = runtime
			f.store.UnlockMutableState()

			// Dispatch pod event
			podHash := deployedHash
			if !test.ptshMatch {
				podHash = nonMatchingHash
			}
			pod := podbuilder.New(f.T(), manifest).
				WithPodID(podID.String()).
				WithTemplateSpecHash(podHash).
				WithPhase("CrashLoopBackOff").
				Build()
			f.podEvent(pod, manifest.Name)

			if test.expectUpdatePodOnManifest {
				f.WaitUntilManifestState("pod on manifest state with updated status", mName,
					func(ms store.ManifestState) bool {
						foundPod, ok := ms.PodWithID(podID)
						return ok && foundPod.Status == "CrashLoopBackOff"
					})
				f.withManifestState(mName, func(ms store.ManifestState) {
					assert.Equal(t, ancestorUID, ms.K8sRuntimeState().PodAncestorUID,
						"expect k8s runtime state to have current pod ancestor UID")
				})
			} else {
				time.Sleep(10 * time.Millisecond)
				f.withManifestState(mName, func(ms store.ManifestState) {
					_, ok := ms.PodWithID(podID)
					assert.False(t, ok, "expect manifest to NOT have pod with ID %q", podID)
				})
			}

			if test.haveAdditionalPod {
				f.withManifestState(mName, func(ms store.ManifestState) {
					_, ok := ms.PodWithID(otherPodID)
					assert.True(t, ok, "expect manifest to have pod with ID %q", otherPodID)
				})
			}

			assert.NoError(t, f.Stop())
			f.assertAllBuildsConsumed()
		})
	}
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

	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("dignore.txt"))
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

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pb := podbuilder.New(f.T(), manifest)
	pod := pb.Build()
	f.startPod(pod, name)
	f.podLog(pod, name, "first string")

	f.upper.store.Dispatch(fswatch.NewTargetFilesChangedAction(manifest.ImageTargetAt(0).ID(), "/go/a.go"))

	f.waitForCompletedBuildCount(2)
	f.podLog(pod, name, "second string")

	f.withState(func(state store.EngineState) {
		ms, _ := state.ManifestState(name)
		spanID := ms.MostRecentPod().SpanID
		assert.Equal(t, "first string\nsecond string\n", state.LogStore.SpanLog(spanID))
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestUpperPodLogInCrashLoopThirdInstanceStillUp(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pb := podbuilder.New(f.T(), manifest)
	f.startPod(pb.Build(), name)
	f.podLog(pb.Build(), name, "first string")
	pb = f.restartPod(pb)
	f.podLog(pb.Build(), name, "second string")
	pb = f.restartPod(pb)
	f.podLog(pb.Build(), name, "third string")

	// the third instance is still up, so we want to show the log from the last crashed pod plus the log from the current pod
	f.withState(func(es store.EngineState) {
		ms, _ := es.ManifestState(name)
		pod := ms.MostRecentPod()
		assert.Contains(t, es.LogStore.SpanLog(pod.SpanID), "third string\n")
		assert.Contains(t, es.LogStore.ManifestLog(name), "second string\n")
		assert.Contains(t, es.LogStore.ManifestLog(name), "third string\n")
		assert.Equal(t, ms.CrashLog.String(), "first string\nsecond string\n")
		assert.Contains(t, es.LogStore.SpanLog(pod.SpanID), "third string\n")
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestUpperPodLogInCrashLoopPodCurrentlyDown(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pb := podbuilder.New(f.T(), manifest)
	f.startPod(pb.Build(), name)
	f.podLog(pb.Build(), name, "first string")
	pb = f.restartPod(pb)
	f.podLog(pb.Build(), name, "second string")

	pod := pb.Build()
	pod.Status.ContainerStatuses[0].Ready = false
	f.notifyAndWaitForPodStatus(pod, name, func(pod store.Pod) bool {
		return !pod.AllContainersReady()
	})

	f.withState(func(state store.EngineState) {
		ms, _ := state.ManifestState(name)
		spanID := ms.MostRecentPod().SpanID
		assert.Equal(t, "first string\nsecond string\n", state.LogStore.SpanLog(spanID))
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestUpperPodRestartsBeforeTiltStart(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("fe")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pb := podbuilder.New(f.T(), manifest).
		WithRestartCount(1)
	f.startPod(pb.Build(), manifest.Name)

	f.withManifestState(name, func(ms store.ManifestState) {
		assert.Equal(t, 1, ms.MostRecentPod().BaselineRestarts)
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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{podbuilder.FakeContainerID()}
	pod := podbuilder.New(f.T(), manifest).Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, pod.Name, call.oneImageState().OneContainerInfo().PodID.String())
	f.waitForCompletedBuildCount(2)
	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagChangedFiles, ms.LastBuild().Reason)
		assert.Equal(t, podbuilder.FakeContainerIDSet(1), ms.LiveUpdatedContainerIDs)
	})

	// Restart the pod with a new container id, to simulate a container restart.
	f.podEvent(podbuilder.New(t, manifest).
		WithPodID("pod-id").
		WithContainerID("funnyContainerID").
		Build(), manifest.Name)
	call = f.nextCall()
	assert.True(t, call.oneImageState().OneContainerInfo().Empty())
	f.waitForCompletedBuildCount(3)

	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagCrash, ms.LastBuild().Reason)
	})

	// kick off another build
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main2.go"))

	// at this point we have not received a pod event for pod that was started by the crash-rebuild,
	// so we should be waiting on the pod deploy to rebuild.
	f.assertNoCall()
	f.withState(func(st store.EngineState) {
		_, holds := buildcontrol.NextTargetToBuild(st)
		assert.Equal(t, store.HoldWaitingForDeploy, holds[manifest.Name])
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestUpperRecordPodWithMultipleContainers(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("foobar")
	manifest := f.newManifest(name.String())

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pod := podbuilder.New(f.T(), manifest).Build()
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
		Name:        "sidecar",
		Image:       "sidecar-image",
		Ready:       false,
		ContainerID: "docker://sidecar",
	})

	f.startPod(pod, manifest.Name)
	f.notifyAndWaitForPodStatus(pod, manifest.Name, func(pod store.Pod) bool {
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

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pod := podbuilder.New(f.T(), manifest).Build()
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
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

	f.startPod(pod, manifest.Name)
	f.notifyAndWaitForPodStatus(pod, manifest.Name, func(pod store.Pod) bool {
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

func TestUpper_ServiceEvent(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("foobar")

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	result := f.b.resultsByID[manifest.K8sTarget().ID()]
	uid := result.(store.K8sBuildResult).DeployedUIDs[0]
	svc := servicebuilder.New(t, manifest).WithUID(uid).WithPort(8080).WithIP("1.2.3.4").Build()
	err := k8swatch.DispatchServiceChange(f.store, svc, manifest.Name, "")
	require.NoError(t, err)

	f.WaitUntilManifestState("lb updated", "foobar", func(ms store.ManifestState) bool {
		return len(ms.K8sRuntimeState().LBs) > 0
	})

	err = f.Stop()
	assert.NoError(t, err)

	ms, _ := f.upper.store.RLockState().ManifestState(manifest.Name)
	defer f.upper.store.RUnlockState()
	lbs := ms.K8sRuntimeState().LBs
	assert.Equal(t, 1, len(lbs))
	url, ok := lbs[k8s.ServiceName(svc.Name)]
	if !ok {
		t.Fatalf("%v did not contain key 'myservice'", lbs)
	}
	assert.Equal(t, "http://1.2.3.4:8080/", url.String())
}

func TestUpper_ServiceEventRemovesURL(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("foobar")

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	result := f.b.resultsByID[manifest.K8sTarget().ID()]
	uid := result.(store.K8sBuildResult).DeployedUIDs[0]
	sb := servicebuilder.New(t, manifest).WithUID(uid).WithPort(8080).WithIP("1.2.3.4")
	svc := sb.Build()
	err := k8swatch.DispatchServiceChange(f.store, svc, manifest.Name, "")
	require.NoError(t, err)

	f.WaitUntilManifestState("lb url added", "foobar", func(ms store.ManifestState) bool {
		url := ms.K8sRuntimeState().LBs[k8s.ServiceName(svc.Name)]
		if url == nil {
			return false
		}
		return "http://1.2.3.4:8080/" == url.String()
	})

	svc = sb.WithIP("").Build()
	err = k8swatch.DispatchServiceChange(f.store, svc, manifest.Name, "")
	require.NoError(t, err)

	f.WaitUntilManifestState("lb url removed", "foobar", func(ms store.ManifestState) bool {
		url := ms.K8sRuntimeState().LBs[k8s.ServiceName(svc.Name)]
		return url == nil
	})

	err = f.Stop()
	assert.NoError(t, err)
}

func TestUpper_PodLogs(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name))

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	pod := podbuilder.New(f.T(), manifest).Build()
	f.startPod(pod, name)
	f.podLog(pod, name, "Hello world!\n")

	err := f.Stop()
	assert.NoError(t, err)
}

func TestK8sEventGlobalLogAndManifestLog(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name))

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	objRef := v1.ObjectReference{UID: f.lastDeployedUID(name)}
	warnEvt := &v1.Event{
		InvolvedObject: objRef,
		Message:        "something has happened zomg",
		Type:           v1.EventTypeWarning,
		ObjectMeta:     metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: f.Now()}},
	}
	f.kClient.EmitEvent(f.ctx, warnEvt)

	f.WaitUntil("event message appears in manifest log", func(st store.EngineState) bool {
		return strings.Contains(st.LogStore.ManifestLog(name), "something has happened zomg")
	})

	f.withState(func(st store.EngineState) {
		assert.Contains(t, st.LogStore.String(), "something has happened zomg", "event message not in global log")
	})

	err := f.Stop()
	assert.NoError(t, err)
}

func TestK8sEventNotLoggedIfNoManifestForUID(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	name := model.ManifestName("fe")
	manifest := f.newManifest(string(name))

	f.Start([]model.Manifest{manifest})
	f.waitForCompletedBuildCount(1)

	warnEvt := &v1.Event{
		InvolvedObject: v1.ObjectReference{UID: types.UID("someRandomUID")},
		Message:        "something has happened zomg",
		Type:           v1.EventTypeWarning,
		ObjectMeta:     metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: f.Now()}},
	}
	f.kClient.EmitEvent(f.ctx, warnEvt)

	time.Sleep(10 * time.Millisecond)

	assert.NotContains(t, f.log.String(), "something has happened zomg",
		"should not log event message b/c it doesn't have a UID -> Manifest mapping")
}

func TestHudExitNoError(t *testing.T) {
	f := newTestFixture(t)
	f.Start([]model.Manifest{})
	f.store.Dispatch(hud.NewExitAction(nil))
	err := f.WaitForExit()
	assert.NoError(t, err)
}

func TestHudExitWithError(t *testing.T) {
	f := newTestFixture(t)
	f.Start([]model.Manifest{})
	e := errors.New("helllllo")
	f.store.Dispatch(hud.NewExitAction(e))
	_ = f.WaitForNoExit()
}

func TestNewConfigsAreWatchedAfterFailure(t *testing.T) {
	f := newTestFixture(t)
	f.loadAndStart()

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

	f.Start([]model.Manifest{redis, server})
	call := f.nextCall()
	assert.True(t, call.dcState().IsEmpty())
	assert.False(t, call.dc().ID().Empty())
	assert.Equal(t, redis.DockerComposeTarget().ID(), call.dc().ID())
	call = f.nextCall()
	assert.True(t, call.dcState().IsEmpty())
	assert.False(t, call.dc().ID().Empty())
	assert.Equal(t, server.DockerComposeTarget().ID(), call.dc().ID())
}

func TestDockerComposeRedeployFromFileChange(t *testing.T) {
	f := newTestFixture(t)
	_, m := f.setupDCFixture()

	f.Start([]model.Manifest{m})

	// Initial build
	call := f.nextCall()
	assert.True(t, call.dcState().IsEmpty())

	// Change a file -- should trigger build
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("package.json"))
	call = f.nextCall()
	assert.Equal(t, []string{f.JoinPath("package.json")}, call.dcState().FilesChanged())
}

// TODO(maia): TestDockerComposeEditConfigFiles once DC manifests load faster (http://bit.ly/2RBX4g5)

func TestDockerComposeEventSetsStatus(t *testing.T) {
	f := newTestFixture(t)
	_, m := f.setupDCFixture()

	f.Start([]model.Manifest{m})
	f.waitForCompletedBuildCount(1)

	// Send event corresponding to status = "In Progress"
	f.dispatchDCEvent(m, dockercompose.ActionCreate, docker.NewCreatedContainerState())

	f.WaitUntilManifestState("resource status = 'In Progress'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCRuntimeState().RuntimeStatus() == model.RuntimeStatusPending
	})

	beforeStart := f.Now()

	// Send event corresponding to status = "OK"
	f.dispatchDCEvent(m, dockercompose.ActionStart, docker.NewRunningContainerState())

	f.WaitUntilManifestState("resource status = 'OK'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCRuntimeState().RuntimeStatus() == model.RuntimeStatusOK
	})

	f.withManifestState(m.ManifestName(), func(ms store.ManifestState) {
		startTime := ms.DCRuntimeState().StartTime
		assert.True(t, store.AfterOrEqual(startTime, beforeStart))
	})

	// An event unrelated to status shouldn't change the status
	f.dispatchDCEvent(m, dockercompose.ActionExecCreate, docker.NewRunningContainerState())

	time.Sleep(10 * time.Millisecond)
	f.WaitUntilManifestState("resource status = 'OK'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCRuntimeState().RuntimeStatus() == model.RuntimeStatusOK
	})
}

func TestDockerComposeStartsEventWatcher(t *testing.T) {
	f := newTestFixture(t)
	_, m := f.setupDCFixture()

	// Actual behavior is that we init with zero manifests, and add in manifests
	// after Tiltfile loads. Mimic that here.
	f.Start([]model.Manifest{})
	time.Sleep(10 * time.Millisecond)

	f.store.Dispatch(configs.ConfigsReloadedAction{
		Manifests:  []model.Manifest{m},
		FinishTime: f.Now(),
	})
	f.waitForCompletedBuildCount(1)

	// Is DockerComposeEventWatcher watching for events??
	f.dispatchDCEvent(m, dockercompose.ActionCreate, docker.NewCreatedContainerState())

	f.WaitUntilManifestState("resource status = 'In Progress'", m.ManifestName(), func(ms store.ManifestState) bool {
		return ms.DCRuntimeState().RuntimeStatus() == model.RuntimeStatusPending
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
		assert.Contains(t, st.LogStore.String(), expected)

		ms, _ := st.ManifestState(m.ManifestName())
		spanID := ms.LastBuild().SpanID
		assert.Contains(t, st.LogStore.SpanLog(spanID), expected)
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

	f.WaitUntil("wait until manifest state has a log", func(state store.EngineState) bool {
		ms, _ := state.ManifestState(m.ManifestName())
		spanID := ms.DCRuntimeState().SpanID
		return spanID != "" && state.LogStore.SpanLog(spanID) != ""
	})

	// recorded on manifest state
	f.withState(func(es store.EngineState) {
		ms, _ := es.ManifestState(m.ManifestName())
		spanID := ms.DCRuntimeState().SpanID
		assert.Contains(t, es.LogStore.SpanLog(spanID), expected)
		assert.Equal(t, 1, strings.Count(es.LogStore.ManifestLog(m.ManifestName()), expected))
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
	f.withState(func(es store.EngineState) {
		ms, _ := es.ManifestState(m.ManifestName())
		spanID := ms.DCRuntimeState().SpanID
		assert.NotContains(t, es.LogStore.SpanLog(spanID), expected)
	})
}

// NOTE(nick): The weird structure of this test is vesigial from when
// we inferred crash from ContainerState rather than sequences of events.
func TestDockerComposeDetectsCrashes(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := f.setupDCFixture()

	f.loadAndStart()
	f.waitForCompletedBuildCount(2)

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, model.RuntimeStatusError, st.DCRuntimeState().RuntimeStatus())
	})

	f.withManifestState(m2.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, model.RuntimeStatusError, st.DCRuntimeState().RuntimeStatus())
	})

	for _, action := range []dockercompose.Action{
		dockercompose.ActionKill,
		dockercompose.ActionKill,
		dockercompose.ActionDie,
		dockercompose.ActionStop,
		dockercompose.ActionRename,
		dockercompose.ActionCreate,
		dockercompose.ActionStart,
		dockercompose.ActionDie,
	} {
		if action == dockercompose.ActionDie {
			f.dispatchDCEvent(m1, action, docker.NewExitErrorContainerState())
		} else {
			f.dispatchDCEvent(m1, action, docker.NewCreatedContainerState())
		}
	}

	f.WaitUntilManifestState("is crashing", m1.ManifestName(), func(st store.ManifestState) bool {
		return st.DCRuntimeState().RuntimeStatus() == model.RuntimeStatusError
	})

	f.withManifestState(m2.ManifestName(), func(st store.ManifestState) {
		assert.NotEqual(t, model.RuntimeStatusError, st.DCRuntimeState().RuntimeStatus())
	})

	for _, action := range []dockercompose.Action{
		dockercompose.ActionKill,
		dockercompose.ActionKill,
		dockercompose.ActionDie,
		dockercompose.ActionStop,
		dockercompose.ActionRename,
		dockercompose.ActionCreate,
		dockercompose.ActionStart,
	} {
		f.dispatchDCEvent(m1, action, docker.NewRunningContainerState())
	}

	f.WaitUntilManifestState("is not crashing", m1.ManifestName(), func(st store.ManifestState) bool {
		return st.DCRuntimeState().RuntimeStatus() == model.RuntimeStatusOK
	})
}

func TestDockerComposeBuildCompletedSetsStatusToUpIfSuccessful(t *testing.T) {
	f := newTestFixture(t)
	m1, _ := f.setupDCFixture()

	expected := container.ID("aaaaaa")
	f.b.nextDockerComposeContainerID = expected

	containerState := docker.NewRunningContainerState()
	f.b.nextDockerComposeContainerState = &containerState

	f.loadAndStart()

	f.waitForCompletedBuildCount(2)

	f.withManifestState(m1.ManifestName(), func(st store.ManifestState) {
		state, ok := st.RuntimeState.(dockercompose.State)
		if !ok {
			t.Fatal("expected RuntimeState to be docker compose, but it wasn't")
		}
		assert.Equal(t, expected, state.ContainerID)
		assert.Equal(t, model.RuntimeStatusOK, state.RuntimeStatus())
	})
}

func TestEmptyTiltfile(t *testing.T) {
	f := newTestFixture(t)
	f.WriteFile("Tiltfile", "")

	closeCh := make(chan error)
	go func() {
		err := f.upper.Start(f.ctx, []string{}, model.TiltBuild{}, store.EngineModeUp,
			f.JoinPath("Tiltfile"), store.TerminalModeHUD,
			analytics.OptIn, token.Token("unit test token"),
			"nonexistent.example.com")
		closeCh <- err
	}()
	f.WaitUntil("build is set", func(st store.EngineState) bool {
		return !st.TiltfileState.LastBuild().Empty()
	})
	f.withState(func(st store.EngineState) {
		assert.Contains(t, st.TiltfileState.LastBuild().Error.Error(), "No resources found. Check out ")
		assertContainsOnce(t, st.LogStore.String(), "No resources found. Check out ")
		assertContainsOnce(t, st.LogStore.ManifestLog(store.TiltfileManifestName), "No resources found. Check out ")

		buildRecord := st.TiltfileState.LastBuild()
		assertContainsOnce(t, st.LogStore.SpanLog(buildRecord.SpanID), "No resources found. Check out ")
	})

	f.cancel()

	err := <-closeCh
	testutils.FailOnNonCanceledErr(t, err, "upper.Start failed")
}

func TestUpperStart(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	tok := token.Token("unit test token")
	cloudAddress := "nonexistent.example.com"

	closeCh := make(chan error)

	f.WriteFile("Tiltfile", "")
	go func() {
		err := f.upper.Start(f.ctx, []string{"foo", "bar"}, model.TiltBuild{},
			store.EngineModeUp, f.JoinPath("Tiltfile"), store.TerminalModeHUD,
			analytics.OptIn, tok, cloudAddress)
		closeCh <- err
	}()
	f.WaitUntil("init action processed", func(state store.EngineState) bool {
		return !state.TiltStartTime.IsZero()
	})

	f.withState(func(state store.EngineState) {
		require.Equal(t, []string{"foo", "bar"}, state.UserConfigState.Args)
		require.Equal(t, f.JoinPath("Tiltfile"), state.TiltfilePath)
		require.Equal(t, tok, state.Token)
		require.Equal(t, analytics.OptIn, state.AnalyticsEffectiveOpt())
		require.Equal(t, cloudAddress, state.CloudAddress)
	})

	f.cancel()

	err := <-closeCh
	testutils.FailOnNonCanceledErr(t, err, "upper.Start failed")
}

func TestWatchManifestsWithCommonAncestor(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithCommonAncestor(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	f.WriteFile(filepath.Join("common", "a.txt"), "hello world")

	aPath := f.JoinPath("common", "a.txt")
	f.fsWatcher.Events <- watch.NewFileEvent(aPath)

	f.waitForCompletedBuildCount(4)

	// Make sure that both builds are triggered, and that they
	// are triggered in a particular order.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	state := call.state[m1.ImageTargets[0].ID()]
	assert.Equal(t, map[string]bool{aPath: true}, state.FilesChangedSet)

	// Make sure that when the second build is triggered, we did the bookkeeping
	// correctly around reusing the image and propagating DepsChanged when
	// we deploy the second k8s target.
	call = f.nextCall("m2 build2")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	id := m2.ImageTargets[0].ID()
	result := f.b.resultsByID[id]
	assert.Equal(t, store.NewBuildState(result, nil, nil), call.state[id])

	id = m2.ImageTargets[1].ID()
	result = f.b.resultsByID[id]

	// Assert the 2nd image was not re-used from the previous result.
	assert.NotEqual(t, result, call.state[id].LastResult)
	assert.Equal(t, map[model.TargetID]bool{m2.ImageTargets[0].ID(): true},
		call.state[id].DepsChangedSet)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestConfigChangeThatChangesManifestIsIncludedInManifestsChangedFile(t *testing.T) {
	// https://app.clubhouse.io/windmill/story/5701/test-testconfigchangethatchangesmanifestisincludedinmanifestschangedfile-is-flaky
	t.Skip("TODO(nick): fix this")

	f := newTestFixture(t)
	defer f.TearDown()

	tiltfile := `
docker_build('gcr.io/windmill-public-containers/servantes/snack', '.')
k8s_yaml('snack.yaml')`
	f.WriteFile("Tiltfile", tiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:dev`)
	f.WriteFile("snack.yaml", testyaml.Deployment("snack", "gcr.io/windmill-public-containers/servantes/snack:old"))

	f.loadAndStart()

	f.waitForCompletedBuildCount(1)

	f.WriteFile("snack.yaml", testyaml.Deployment("snack", "gcr.io/windmill-public-containers/servantes/snack:new"))
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("snack.yaml"))

	f.waitForCompletedBuildCount(2)

	f.withManifestState("snack", func(ms store.ManifestState) {
		require.Equal(t, []string{f.JoinPath("snack.yaml")}, ms.LastBuild().Edits)
	})

	f.WriteFile("Dockerfile", `FROM iron/go:foobar`)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("Dockerfile"))

	f.waitForCompletedBuildCount(3)

	f.withManifestState("snack", func(ms store.ManifestState) {
		require.Equal(t, []string{f.JoinPath("Dockerfile")}, ms.LastBuild().Edits)
	})
}

func TestSetAnalyticsOpt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	opt := func(ia InitAction) InitAction {
		ia.AnalyticsUserOpt = analytics.OptIn
		return ia
	}

	f.Start([]model.Manifest{}, opt)
	f.store.Dispatch(store.AnalyticsUserOptAction{Opt: analytics.OptOut})
	f.WaitUntil("opted out", func(state store.EngineState) bool {
		return state.AnalyticsEffectiveOpt() == analytics.OptOut
	})

	// if we don't wait for 1 here, it's possible the state flips to out and back to in before the subscriber sees it,
	// and we end up with no events
	f.opter.WaitUntilCount(t, 1)

	f.store.Dispatch(store.AnalyticsUserOptAction{Opt: analytics.OptIn})
	f.WaitUntil("opted in", func(state store.EngineState) bool {
		return state.AnalyticsEffectiveOpt() == analytics.OptIn
	})

	f.opter.WaitUntilCount(t, 2)

	err := f.Stop()
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, []analytics.Opt{analytics.OptOut, analytics.OptIn}, f.opter.Calls())
}

func TestFeatureFlagsStoredOnState(t *testing.T) {
	f := newTestFixture(t)

	f.Start([]model.Manifest{})

	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime: f.Now(),
		Features:   map[string]bool{"foo": true},
	})

	f.WaitUntil("feature is enabled", func(state store.EngineState) bool {
		return state.Features["foo"] == true
	})

	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime: f.Now(),
		Features:   map[string]bool{"foo": false},
	})

	f.WaitUntil("feature is disabled", func(state store.EngineState) bool {
		return state.Features["foo"] == false
	})
}

func TestTeamIDStoredOnState(t *testing.T) {
	f := newTestFixture(t)

	f.Start([]model.Manifest{})

	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime: f.Now(),
		TeamID:     "sharks",
	})

	f.WaitUntil("teamID is set to sharks", func(state store.EngineState) bool {
		return state.TeamID == "sharks"
	})

	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime: f.Now(),
		TeamID:     "jets",
	})

	f.WaitUntil("teamID is set to jets", func(state store.EngineState) bool {
		return state.TeamID == "jets"
	})
}

func TestBuildLogAction(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.bc.DisableForTesting()

	manifest := f.newManifest("alert-injester")
	f.Start([]model.Manifest{manifest})

	f.store.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName: manifest.Name,
		StartTime:    f.Now(),
		SpanID:       SpanIDForBuildLog(1),
	})

	f.store.Dispatch(store.NewLogAction(manifest.Name, SpanIDForBuildLog(1), logger.InfoLvl, nil, []byte(`a
bc
def
ghij`)))

	f.WaitUntil("log appears", func(es store.EngineState) bool {
		ms, _ := es.ManifestState("alert-injester")
		spanID := ms.CurrentBuild.SpanID
		return spanID != "" && len(es.LogStore.SpanLog(spanID)) > 0
	})

	f.withState(func(s store.EngineState) {
		assert.Contains(t, s.LogStore.String(), `alert-injest… │ a
alert-injest… │ bc
alert-injest… │ def
alert-injest… │ ghij`)
	})

	err := f.Stop()
	assert.Nil(t, err)
}

func TestBuildErrorLoggedOnceByUpper(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("alert-injester")
	err := errors.New("cats and dogs, living together")
	f.b.nextBuildError = err

	f.Start([]model.Manifest{manifest})

	f.waitForCompletedBuildCount(1)

	// so the test name says "once", but the fake builder also logs once, so we get it twice
	f.withState(func(state store.EngineState) {
		require.Equal(t, 2, strings.Count(state.LogStore.String(), err.Error()))
	})
}

func TestTiltfileChangedFilesOnlyLoggedAfterFirstBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile')
k8s_yaml('snack.yaml')`)
	f.WriteFile("Dockerfile", `FROM iron/go:dev1`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.WriteFile("src/main.go", "hello")

	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})

	// we shouldn't log changes for first build
	f.withState(func(state store.EngineState) {
		require.NotContains(t, state.LogStore.String(), "changed: [")
	})

	f.WriteFile("Tiltfile", `
docker_build('gcr.io/windmill-public-containers/servantes/snack', './src', dockerfile='Dockerfile', ignore='foo')
k8s_yaml('snack.yaml')`)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("Tiltfile"))

	f.WaitUntil("Tiltfile reloaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 2
	})

	f.withState(func(state store.EngineState) {
		expectedMessage := fmt.Sprintf("1 File Changed: [%s]", f.JoinPath("Tiltfile"))
		require.Contains(t, state.LogStore.String(), expectedMessage)
	})
}

func TestDeployUIDsInEngineState(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	uid := types.UID("fake-uid")
	f.b.nextDeployedUID = uid

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	_ = f.nextCall()
	f.WaitUntilManifestState("UID in ManifestState", "fe", func(state store.ManifestState) bool {
		return state.K8sRuntimeState().DeployedUIDSet.Contains(uid)
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestEnableFeatureOnFail(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
enable_feature('snapshots')
fail('goodnight moon')
`)

	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.True(t, state.Features["snapshots"])
	})
}

func TestEnableMetrics(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
experimental_metrics_settings(enabled=True)
fail('goodnight moon')
`)

	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.True(t, state.MetricsSettings.Enabled)
	})

	f.WriteFile("Tiltfile", `
experimental_metrics_settings(enabled=False)
k8s_yaml('snack.yaml')
`)
	f.WriteFile("snack.yaml", simpleYAML)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("Tiltfile"))

	f.WaitUntil("Tiltfile reloaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 2
	})
	f.withState(func(state store.EngineState) {
		assert.False(t, state.MetricsSettings.Enabled)
	})
}

func TestSecretScrubbed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	tiltfile := `
print('about to print secret')
print('aGVsbG8=')
k8s_yaml('secret.yaml')`
	f.WriteFile("Tiltfile", tiltfile)
	f.WriteFile("secret.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
data:
  client-secret: aGVsbG8=
`)

	f.loadAndStart()

	f.waitForCompletedBuildCount(1)

	f.withState(func(state store.EngineState) {
		log := state.LogStore.String()
		assert.Contains(t, log, "about to print secret")
		assert.NotContains(t, log, "aGVsbG8=")
		assert.Contains(t, log, "[redacted secret my-secret:client-secret]")
	})
}

func TestShortSecretNotScrubbed(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	tiltfile := `
print('about to print secret: s')
k8s_yaml('secret.yaml')`
	f.WriteFile("Tiltfile", tiltfile)
	f.WriteFile("secret.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
stringData:
  client-secret: s
`)

	f.loadAndStart()

	f.waitForCompletedBuildCount(1)

	f.withState(func(state store.EngineState) {
		log := state.LogStore.String()
		assert.Contains(t, log, "about to print secret: s")
		assert.NotContains(t, log, "redacted")
	})
}

func TestDisableDockerPrune(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.WriteFile("Tiltfile", `
docker_prune_settings(disable=True)
`+simpleTiltfile)

	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.False(t, state.DockerPruneSettings.Enabled)
	})
}

func TestDockerPruneEnabledByDefault(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", simpleTiltfile)
	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.True(t, state.DockerPruneSettings.Enabled)
		assert.Equal(t, model.DockerPruneDefaultMaxAge, state.DockerPruneSettings.MaxAge)
		assert.Equal(t, model.DockerPruneDefaultInterval, state.DockerPruneSettings.Interval)
	})
}

func TestHasEverBeenReadyK8s(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	m := f.newManifest("foobar")
	f.Start([]model.Manifest{m})

	f.waitForCompletedBuildCount(1)
	f.withManifestState(m.Name, func(ms store.ManifestState) {
		require.False(t, ms.RuntimeState.HasEverBeenReadyOrSucceeded())
	})

	pb := podbuilder.New(t, m).WithContainerReady(true)
	f.podEvent(pb.Build(), m.Name)
	f.WaitUntilManifestState("flagged ready", m.Name, func(state store.ManifestState) bool {
		return state.RuntimeState.HasEverBeenReadyOrSucceeded()
	})
}

func TestHasEverBeenCompleteK8s(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	m := f.newManifest("foobar")
	f.Start([]model.Manifest{m})

	f.waitForCompletedBuildCount(1)
	f.withManifestState(m.Name, func(ms store.ManifestState) {
		require.False(t, ms.RuntimeState.HasEverBeenReadyOrSucceeded())
	})

	pb := podbuilder.New(t, m).WithPhase(string(v1.PodSucceeded))
	f.podEvent(pb.Build(), m.Name)
	f.WaitUntilManifestState("flagged ready", m.Name, func(state store.ManifestState) bool {
		return state.RuntimeState.HasEverBeenReadyOrSucceeded()
	})
}

func TestHasEverBeenReadyLocal(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	m := manifestbuilder.New(f, "foobar").WithLocalResource("foo", []string{f.Path()}).Build()
	f.b.nextBuildError = errors.New("failure!")
	f.Start([]model.Manifest{m})

	// first build will fail, HasEverBeenReadyOrSucceeded should be false
	f.waitForCompletedBuildCount(1)
	f.withManifestState(m.Name, func(ms store.ManifestState) {
		require.False(t, ms.RuntimeState.HasEverBeenReadyOrSucceeded())
	})

	// second build will succeed, HasEverBeenReadyOrSucceeded should be true
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("bar", "main.go"))
	f.WaitUntilManifestState("flagged ready", m.Name, func(state store.ManifestState) bool {
		return state.RuntimeState.HasEverBeenReadyOrSucceeded()
	})
}

func TestHasEverBeenReadyDC(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	m, _ := f.setupDCFixture()
	f.Start([]model.Manifest{m})

	f.waitForCompletedBuildCount(1)
	f.withManifestState(m.Name, func(ms store.ManifestState) {
		require.NotNil(t, ms.RuntimeState)
		require.False(t, ms.RuntimeState.HasEverBeenReadyOrSucceeded())
	})

	// second build will succeed, HasEverBeenReadyOrSucceeded should be true
	f.dispatchDCEvent(m, dockercompose.ActionStart, docker.NewRunningContainerState())

	f.WaitUntilManifestState("flagged ready", m.Name, func(state store.ManifestState) bool {
		return state.RuntimeState.HasEverBeenReadyOrSucceeded()
	})
}

func TestVersionSettingsStoredOnState(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.Start([]model.Manifest{})

	vs := model.VersionSettings{
		CheckUpdates: false,
	}
	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime:      f.Now(),
		VersionSettings: vs,
	})

	f.WaitUntil("CheckVersionUpdates is set to false", func(state store.EngineState) bool {
		return state.VersionSettings.CheckUpdates == false
	})
}

func TestAnalyticsTiltfileOpt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.Start([]model.Manifest{})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, analytics.OptDefault, state.AnalyticsEffectiveOpt())
	})

	f.store.Dispatch(configs.ConfigsReloadedAction{
		FinishTime:           f.Now(),
		AnalyticsTiltfileOpt: analytics.OptIn,
	})

	f.WaitUntil("analytics tiltfile opt-in", func(state store.EngineState) bool {
		return state.AnalyticsTiltfileOpt == analytics.OptIn
	})

	f.withState(func(state store.EngineState) {
		assert.Equal(t, analytics.OptIn, state.AnalyticsEffectiveOpt())
	})
}

func TestConfigArgsChangeCausesTiltfileRerun(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
print('hello')
config.define_string_list('foo')
cfg = config.parse()
print('foo=', cfg['foo'])`)

	opt := func(ia InitAction) InitAction {
		ia.UserArgs = []string{"--foo", "bar"}
		return ia
	}

	f.loadAndStart(opt)

	f.WaitUntil("first tiltfile build finishes", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})

	f.withState(func(state store.EngineState) {
		spanID := state.TiltfileState.LastBuild().SpanID
		require.Contains(t, state.LogStore.SpanLog(spanID), `foo= ["bar"]`)
	})
	f.store.Dispatch(server.SetTiltfileArgsAction{Args: []string{"--foo", "baz", "--foo", "quu"}})

	f.WaitUntil("second tiltfile build finishes", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 2
	})

	f.withState(func(state store.EngineState) {
		spanID := state.TiltfileState.LastBuild().SpanID
		require.Contains(t, state.LogStore.SpanLog(spanID), `foo= ["baz", "quu"]`)
	})
}

func TestTelemetryLogAction(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.Start([]model.Manifest{})

	f.store.Dispatch(store.NewLogAction(model.TiltfileManifestName, "0", logger.InfoLvl, nil, []byte("testing")))

	f.WaitUntil("log is stored", func(state store.EngineState) bool {
		l := state.LogStore.ManifestLog(store.TiltfileManifestName)
		return strings.Contains(l, "testing")
	})
}

func TestLocalResourceServeWithNoUpdate(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	m := manifestbuilder.New(f, "foo").
		WithLocalServeCmd("true").
		Build()

	f.Start([]model.Manifest{m})

	f.WaitUntil("resource is served", func(state store.EngineState) bool {
		return strings.Contains(state.LogStore.ManifestLog(m.Name), "Starting cmd true")
	})

	err := f.Stop()
	require.NoError(t, err)
}

func TestLocalResourceServeChangeCmd(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", "local_resource('foo', serve_cmd='true')")

	f.loadAndStart()

	f.WaitUntil("true is served", func(state store.EngineState) bool {
		return strings.Contains(state.LogStore.ManifestLog("foo"), "Starting cmd true")
	})

	f.WriteFile("Tiltfile", "local_resource('foo', serve_cmd='false')")
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("Tiltfile"))

	f.WaitUntil("false is served", func(state store.EngineState) bool {
		return strings.Contains(state.LogStore.ManifestLog("foo"), "Starting cmd false")
	})

	f.fe.RequireNoKnownProcess(t, "true")

	err := f.Stop()
	require.NoError(t, err)
}

func TestDefaultUpdateSettings(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.WriteFile("Tiltfile", simpleTiltfile)

	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.Equal(t, model.DefaultUpdateSettings(), state.UpdateSettings)
	})
}

func TestSetK8sUpsertTimeout(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.WriteFile("Tiltfile", `
update_settings(k8s_upsert_timeout_secs=123)
`+simpleTiltfile)
	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.Equal(t, 123*time.Second, state.UpdateSettings.K8sUpsertTimeout())
	})
}

func TestSetMaxBuildSlots(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", `FROM iron/go:prod`)
	f.WriteFile("snack.yaml", simpleYAML)

	f.WriteFile("Tiltfile", `
update_settings(max_parallel_updates=123)
`+simpleTiltfile)
	f.loadAndStart()

	f.WaitUntil("Tiltfile loaded", func(state store.EngineState) bool {
		return len(state.TiltfileState.BuildHistory) == 1
	})
	f.withState(func(state store.EngineState) {
		assert.Equal(t, 123, state.UpdateSettings.MaxParallelUpdates())
	})
}

// https://github.com/tilt-dev/tilt/issues/3514
func TestTiltignoreRespectedOnError(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `local("echo hi > a.txt")
read_file('a.txt')
fail('x')`)
	f.WriteFile(".tiltignore", "a.txt")

	f.Init(InitAction{
		EngineMode:   store.EngineModeUp,
		TiltfilePath: f.JoinPath("Tiltfile"),
		TerminalMode: store.TerminalModeHUD,
		StartTime:    f.Now(),
	})

	f.WaitUntil(".tiltignore processed", func(es store.EngineState) bool {
		return strings.Contains(es.TiltIgnoreContents, "a.txt")
	})

	f.WriteFile(".tiltignore", "a.txt\nb.txt\n")
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("Tiltfile"))

	f.WaitUntil(".tiltignore processed", func(es store.EngineState) bool {
		return strings.Contains(es.TiltIgnoreContents, "b.txt")
	})

	err := f.Stop()
	assert.NoError(t, err)
}

type testFixture struct {
	*tempdir.TempDirFixture
	t                          *testing.T
	ctx                        context.Context
	cancel                     func()
	clock                      clockwork.Clock
	upper                      Upper
	b                          *fakeBuildAndDeployer
	fsWatcher                  *fswatch.FakeMultiWatcher
	timerMaker                 *fswatch.FakeTimerMaker
	docker                     *docker.FakeClient
	kClient                    *k8s.FakeK8sClient
	hud                        hud.HeadsUpDisplay
	ts                         *hud.TerminalStream
	upperInitResult            chan error
	log                        *bufsync.ThreadSafeBuffer
	store                      *store.Store
	bc                         *BuildController
	fwm                        *fswatch.WatchManager
	gm                         *fswatch.GitManager
	cc                         *configs.ConfigsController
	dcc                        *dockercompose.FakeDCClient
	tfl                        tiltfile.TiltfileLoader
	opter                      *tiltanalytics.FakeOpter
	dp                         *dockerprune.DockerPruner
	fe                         *local.FakeExecer
	overrideMaxParallelUpdates int

	onchangeCh chan bool
}

func newTestFixture(t *testing.T) *testFixture {
	f := tempdir.NewTempDirFixture(t)

	log := bufsync.NewThreadSafeBuffer()
	to := tiltanalytics.NewFakeOpter(analytics.OptIn)
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsWithOpterForTest(log, to)
	ctx, cancel := context.WithCancel(ctx)

	watcher := fswatch.NewFakeMultiWatcher()
	b := newFakeBuildAndDeployer(t)

	timerMaker := fswatch.MakeFakeTimerMaker(t)

	dockerClient := docker.NewFakeClient()

	kCli := k8s.NewFakeK8sClient()
	of := k8s.ProvideOwnerFetcher(kCli)
	pw := k8swatch.NewPodWatcher(kCli, of)
	sw := k8swatch.NewServiceWatcher(kCli, of)

	fSub := fixtureSub{ch: make(chan bool, 1000)}
	st := store.NewStore(UpperReducer, store.LogActionsFlag(false))
	st.AddSubscriber(ctx, fSub)

	plm := runtimelog.NewPodLogManager(kCli)
	bc := NewBuildController(b)

	err := os.Mkdir(f.JoinPath(".git"), os.FileMode(0777))
	if err != nil {
		t.Fatal(err)
	}

	clock := clockwork.NewRealClock()
	env := k8s.EnvDockerDesktop
	fwm := fswatch.NewWatchManager(watcher.NewSub, timerMaker.Maker())
	gm := fswatch.NewGitManager(watcher.NewSub)
	pfc := portforward.NewController(kCli)
	au := engineanalytics.NewAnalyticsUpdater(ta, engineanalytics.CmdTags{})
	ar := engineanalytics.ProvideAnalyticsReporter(ta, st, kCli, env)
	fakeDcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	k8sContextExt := k8scontext.NewExtension("fake-context", env)
	versionExt := version.NewExtension(model.TiltBuild{Version: "0.5.0"})
	configExt := config.NewExtension("up")
	tfl := tiltfile.ProvideTiltfileLoader(ta, kCli, k8sContextExt, versionExt, configExt, fakeDcc, "localhost", feature.MainDefaults, env)
	cc := configs.NewConfigsController(tfl, dockerClient)
	dcw := dcwatch.NewEventWatcher(fakeDcc, dockerClient)
	dclm := runtimelog.NewDockerComposeLogManager(fakeDcc)
	pm := NewProfilerManager()
	sCli := synclet.NewTestSyncletClient(dockerClient)
	sGRPCCli, err := synclet.FakeGRPCWrapper(ctx, sCli)
	assert.NoError(t, err)
	sm := containerupdate.NewSyncletManagerForTests(kCli, sGRPCCli, sCli)
	hudsc := server.ProvideHeadsUpServerController("localhost", 0, &server.HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	ewm := k8swatch.NewEventWatchManager(kCli, of)
	tcum := cloud.NewStatusManager(httptest.NewFakeClientEmptyJSON(), clock)
	fe := local.NewFakeExecer()
	lc := local.NewController(fe)
	ts := hud.NewTerminalStream(hud.NewIncrementalPrinter(log), st)
	tp := prompt.NewTerminalPrompt(ta, prompt.TTYOpen, prompt.BrowserOpen,
		log, "localhost", model.WebURL{})
	h := hud.NewFakeHud()

	dp := dockerprune.NewDockerPruner(dockerClient)
	dp.DisabledForTesting(true)

	ret := &testFixture{
		TempDirFixture: f,
		t:              t,
		ctx:            ctx,
		cancel:         cancel,
		clock:          clock,
		b:              b,
		fsWatcher:      watcher,
		timerMaker:     &timerMaker,
		docker:         dockerClient,
		kClient:        kCli,
		hud:            h,
		ts:             ts,
		log:            log,
		store:          st,
		bc:             bc,
		onchangeCh:     fSub.ch,
		fwm:            fwm,
		gm:             gm,
		cc:             cc,
		dcc:            fakeDcc,
		tfl:            tfl,
		opter:          to,
		dp:             dp,
		fe:             fe,
	}

	ret.disableEnvAnalyticsOpt()

	tc := telemetry.NewController(clock, tracer.NewSpanCollector(ctx))
	podm := k8srollout.NewPodMonitor()
	ec := exit.NewController()

	de := metrics.NewDeferredExporter()
	mc := metrics.NewController(de, model.TiltBuild{}, "")

	subs := ProvideSubscribers(h, ts, tp, pw, sw, plm, pfc, fwm, gm, bc, cc, dcw, dclm, pm, sm, ar, hudsc, au, ewm, tcum, dp, tc, lc, podm, ec, mc)
	ret.upper = NewUpper(ctx, st, subs)

	go func() {
		err := h.Run(ctx, ret.upper.Dispatch, hud.DefaultRefreshInterval)
		testutils.FailOnNonCanceledErr(t, err, "hud.Run failed")
	}()

	return ret
}

func (f *testFixture) Now() time.Time {
	return f.clock.Now()
}

func (f *testFixture) fakeHud() *hud.FakeHud {
	fakeHud, ok := f.hud.(*hud.FakeHud)
	if !ok {
		f.t.Fatalf("called f.fakeHud() on a test fixure without a fakeHud (instead f.hud is of type: %T", f.hud)
	}
	return fakeHud
}

// starts the upper with the given manifests, bypassing normal tiltfile loading
func (f *testFixture) Start(manifests []model.Manifest, initOptions ...initOption) {
	f.setManifests(manifests)

	ia := InitAction{
		EngineMode:   store.EngineModeUp,
		TiltfilePath: f.JoinPath("Tiltfile"),
		TerminalMode: store.TerminalModeHUD,
		StartTime:    f.Now(),
	}
	for _, o := range initOptions {
		ia = o(ia)
	}
	f.Init(ia)
}

func (f *testFixture) setManifests(manifests []model.Manifest) {
	tfl := tiltfile.NewFakeTiltfileLoader()
	tfl.Result.Manifests = manifests
	f.tfl = tfl
	f.cc.SetTiltfileLoaderForTesting(tfl)
}

func (f *testFixture) setMaxParallelUpdates(n int) {
	f.overrideMaxParallelUpdates = n

	state := f.store.LockMutableStateForTesting()
	state.UpdateSettings = state.UpdateSettings.WithMaxParallelUpdates(n)
	f.store.UnlockMutableState()
}

func (f *testFixture) disableEnvAnalyticsOpt() {
	state := f.store.LockMutableStateForTesting()
	state.AnalyticsEnvOpt = analytics.OptDefault
	f.store.UnlockMutableState()
}

type initOption func(ia InitAction) InitAction

func (f *testFixture) Init(action InitAction) {
	watchFiles := action.EngineMode.WatchesFiles()
	f.upperInitResult = make(chan error, 10)

	go func() {
		err := f.upper.Init(f.ctx, action)
		if err != nil && err != context.Canceled {
			// Print this out here in case the test never completes
			log.Printf("upper exited: %v\n", err)
			f.cancel()
		}
		select {
		case f.upperInitResult <- err:
		default:
			fmt.Println("writing to upperInitResult would block!")
			panic(err)
		}
	}()

	f.WaitUntil("tiltfile build finishes", func(st store.EngineState) bool {
		return !st.TiltfileState.LastBuild().Empty()
	})

	state := f.store.LockMutableStateForTesting()
	expectedWatchCount := len(fswatch.WatchableTargetsForManifests(state.Manifests()))
	if len(state.ConfigFiles) > 0 {
		// watchmanager also creates a watcher for config files
		expectedWatchCount++
	}
	if f.overrideMaxParallelUpdates > 0 {
		state.UpdateSettings = state.UpdateSettings.WithMaxParallelUpdates(f.overrideMaxParallelUpdates)
	}
	f.store.UnlockMutableState()

	f.PollUntil("watches set up", func() bool {
		return !watchFiles || f.fwm.TargetWatchCount() == expectedWatchCount
	})
}

func (f *testFixture) Stop() error {
	f.cancel()
	err := <-f.upperInitResult
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
	case err := <-f.upperInitResult:
		return err
	}
}

func (f *testFixture) WaitForNoExit() error {
	select {
	case <-time.After(time.Second):
		return nil
	case err := <-f.upperInitResult:
		f.T().Fatalf("upper exited when it shouldn't have")
		return err
	}
}

func (f *testFixture) SetNextBuildError(err error) {
	// Before setting the nextBuildError, make sure that any in-flight builds (state.BuildStartedCount)
	// have hit the buildAndDeployer (f.b.buildCount); by the time we've incremented buildCount and
	// the fakeBaD mutex is unlocked, we've already grabbed the nextBuildError for that build,
	// so we can freely set it here for a future build.
	f.WaitUntil("any in-flight builds have hit the buildAndDeployer", func(state store.EngineState) bool {
		f.b.mu.Lock()
		defer f.b.mu.Unlock()
		return f.b.buildCount == state.StartedBuildCount
	})

	_ = f.store.RLockState()
	f.b.mu.Lock()
	f.b.nextBuildError = err
	f.b.mu.Unlock()
	f.store.RUnlockState()
}

func (f *testFixture) SetNextLiveUpdateCompileError(err error, containerIDs []container.ID) {
	f.WaitUntil("any in-flight builds have hit the buildAndDeployer", func(state store.EngineState) bool {
		f.b.mu.Lock()
		defer f.b.mu.Unlock()
		return f.b.buildCount == state.StartedBuildCount
	})

	_ = f.store.RLockState()
	f.b.mu.Lock()
	f.b.nextLiveUpdateCompileError = err
	f.b.nextLiveUpdateContainerIDs = containerIDs
	f.b.mu.Unlock()
	f.store.RUnlockState()
}

// Wait until the given view test passes.
func (f *testFixture) WaitUntilHUD(msg string, isDone func(view.View) bool) {
	f.fakeHud().WaitUntil(f.T(), f.ctx, msg, isDone)
}

func (f *testFixture) WaitUntilHUDResource(msg string, name model.ManifestName, isDone func(view.Resource) bool) {
	f.fakeHud().WaitUntilResource(f.T(), f.ctx, msg, name, isDone)
}

// Wait until the given engine state test passes.
func (f *testFixture) WaitUntil(msg string, isDone func(store.EngineState) bool) {
	f.T().Helper()

	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	isCanceled := false

	for {
		state := f.upper.store.RLockState()
		done := isDone(state)
		f.upper.store.RUnlockState()
		if done {
			return
		}

		if isCanceled {
			f.T().Fatalf("Timed out waiting for: %s", msg)
		}

		select {
		case <-ctx.Done():
			// Let the loop run the isDone test one more time
			isCanceled = true
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

// gets the args for the next BaD call and blocks until that build is reflected in EngineState
func (f *testFixture) nextCallComplete(msgAndArgs ...interface{}) buildAndDeployCall {
	call := f.nextCall(msgAndArgs...)
	f.waitForCompletedBuildCount(call.count)
	return call
}

// gets the args passed to the next call to the BaDer
// note that if you're using this to block until a build happens, it only blocks until the BaDer itself finishes
// so it can return before the build has actually been processed by the upper or the EngineState reflects
// the completed build.
// using `nextCallComplete` will ensure you block until the EngineState reflects the completed build.
func (f *testFixture) nextCall(msgAndArgs ...interface{}) buildAndDeployCall {
	msg := "timed out waiting for BuildAndDeployCall"
	if len(msgAndArgs) > 0 {
		format := msgAndArgs[0].(string)
		args := msgAndArgs[1:]
		msg = fmt.Sprintf("%s: %s", msg, fmt.Sprintf(format, args...))
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

func (f *testFixture) lastDeployedUID(manifestName model.ManifestName) types.UID {
	var manifest model.Manifest
	f.withManifestTarget(manifestName, func(mt store.ManifestTarget) {
		manifest = mt.Manifest
	})
	result := f.b.resultsByID[manifest.K8sTarget().ID()]
	k8sResult, ok := result.(store.K8sBuildResult)
	if !ok {
		return ""
	}
	uids := k8sResult.DeployedUIDs
	if len(uids) > 0 {
		return uids[0]
	}
	return ""
}

func (f *testFixture) startPod(pod *v1.Pod, manifestName model.ManifestName) {
	f.upper.store.Dispatch(k8swatch.NewPodChangeAction(pod, manifestName, f.lastDeployedUID(manifestName)))

	f.WaitUntilManifestState("pod appears", manifestName, func(ms store.ManifestState) bool {
		return ms.MostRecentPod().PodID == k8s.PodID(pod.Name)
	})
}

func (f *testFixture) podLog(pod *v1.Pod, manifestName model.ManifestName, s string) {
	podID := k8s.PodID(pod.Name)
	f.upper.store.Dispatch(store.NewLogAction(manifestName, runtimelog.SpanIDForPod(podID), logger.InfoLvl, nil, []byte(s+"\n")))

	f.WaitUntil("pod log seen", func(es store.EngineState) bool {
		ms, _ := es.ManifestState(manifestName)
		return strings.Contains(es.LogStore.SpanLog(ms.MostRecentPod().SpanID), s)
	})
}

func (f *testFixture) restartPod(pb podbuilder.PodBuilder) podbuilder.PodBuilder {
	restartCount := pb.RestartCount() + 1
	pb = pb.WithRestartCount(restartCount)

	pod := pb.Build()
	mn := pb.ManifestName()
	f.upper.store.Dispatch(k8swatch.NewPodChangeAction(pod, mn, f.lastDeployedUID(mn)))

	f.WaitUntilManifestState("pod restart seen", "foobar", func(ms store.ManifestState) bool {
		return ms.MostRecentPod().AllContainerRestarts() == int(restartCount)
	})
	return pb
}

func (f *testFixture) notifyAndWaitForPodStatus(pod *v1.Pod, mn model.ManifestName, pred func(pod store.Pod) bool) {
	f.upper.store.Dispatch(k8swatch.NewPodChangeAction(pod, mn, f.lastDeployedUID(mn)))

	f.WaitUntilManifestState("pod status change seen", mn, func(state store.ManifestState) bool {
		return pred(state.MostRecentPod())
	})
}

func (f *testFixture) waitForCompletedBuildCount(count int) {
	f.WaitUntil(fmt.Sprintf("%d builds done", count), func(state store.EngineState) bool {
		return state.CompletedBuildCount >= count
	})
}

func (f *testFixture) LogLines() []string {
	return strings.Split(f.log.String(), "\n")
}

func (f *testFixture) TearDown() {
	if f.T().Failed() {
		f.withState(func(es store.EngineState) {
			fmt.Println(es.LogStore.String())
		})
	}
	f.TempDirFixture.TearDown()
	f.kClient.TearDown()
	close(f.fsWatcher.Events)
	close(f.fsWatcher.Errors)
	f.cancel()
}

func (f *testFixture) podEvent(pod *v1.Pod, mn model.ManifestName) {
	f.store.Dispatch(k8swatch.NewPodChangeAction(pod, mn, f.lastDeployedUID(mn)))
}

func (f *testFixture) newManifest(name string) model.Manifest {
	iTarget := NewSanchoLiveUpdateImageTarget(f)
	return manifestbuilder.New(f, model.ManifestName(name)).
		WithK8sYAML(SanchoYAML).
		WithImageTarget(iTarget).
		Build()
}

func (f *testFixture) newManifestWithRef(name string, ref reference.Named) model.Manifest {
	refSel := container.NewRefSelector(ref)

	iTarget := NewSanchoLiveUpdateImageTarget(f)
	iTarget.Refs.ConfigurationRef = refSel

	return manifestbuilder.New(f, model.ManifestName(name)).
		WithK8sYAML(SanchoYAML).
		WithImageTarget(iTarget).
		Build()
}

func (f *testFixture) newDockerBuildManifestWithBuildPath(name string, path string) model.Manifest {
	db := model.DockerBuild{Dockerfile: "FROM alpine", BuildPath: path}
	iTarget := NewSanchoLiveUpdateImageTarget(f).WithBuildDetails(db)
	iTarget.Refs.ConfigurationRef = container.MustParseSelector(strings.ToLower(name)) // each target should have a unique ID
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

func (f *testFixture) loadAndStart(initOptions ...initOption) {
	ia := InitAction{
		EngineMode:   store.EngineModeUp,
		TiltfilePath: f.JoinPath("Tiltfile"),
		TerminalMode: store.TerminalModeHUD,
		StartTime:    f.Now(),
	}
	for _, opt := range initOptions {
		ia = opt(ia)
	}
	f.Init(ia)
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
		f.fsWatcher.Events <- watch.NewFileEvent(filename)
	}

	// The test harness was written for a time when most tests didn't
	// have a Tiltfile. So
	// 1) Tiltfile execution doesn't happen at test startup
	// 2) Because the Tiltfile isn't executed, ConfigFiles isn't populated
	// 3) Because ConfigFiles isn't populated, ConfigsTargetID watches aren't set up properly
	// So just fire a change action manually.
	f.store.Dispatch(fswatch.NewTargetFilesChangedAction(fswatch.ConfigsTargetID, filenames...))
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

	var dcConfig dockercompose.Config
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

	tlr := f.tfl.Load(f.ctx, f.JoinPath("Tiltfile"), model.UserConfigState{})
	if tlr.Error != nil {
		f.T().Fatal(tlr.Error)
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
	res, ok := f.fakeHud().LastView.Resource(name)
	if !ok {
		f.T().Fatalf("Resource not found: %s", name)
	}
	return res
}

// registerDeployedPodTemplateSpecHashToManifest stores the given hash on the k8sRuntimeState for the
// specified manifest (so that we will recognize incoming pod events with the same hash
// as belonging to this manifest).
func (f *testFixture) registerDeployedPodTemplateSpecHashToManifest(name model.ManifestName, ptsh k8s.PodTemplateSpecHash) {
	st := f.store.LockMutableStateForTesting()
	ms, ok := st.ManifestState(name)
	require.True(f.T(), ok, "no manifest found on state matching name %q", name)
	runtime := ms.K8sRuntimeState()
	runtime.DeployedPodTemplateSpecHashSet.Add(ptsh)
	ms.RuntimeState = runtime
	f.store.UnlockMutableState()
}

func (f *testFixture) completeBuildForManifest(m model.Manifest) {
	f.b.completeBuild(targetIDStringForManifest(m))
}

func podTemplateSpecHashesForTarg(t *testing.T, targ model.K8sTarget) []k8s.PodTemplateSpecHash {
	entities, err := k8s.ParseYAMLFromString(targ.YAML)
	require.NoError(t, err)

	var ptsh []k8s.PodTemplateSpecHash
	for _, e := range entities {
		hashes := podTemplateSpecHashes(t, &e)
		ptsh = append(ptsh, hashes...)
	}

	return ptsh
}

func podTemplateSpecHashes(t *testing.T, e *k8s.K8sEntity) []k8s.PodTemplateSpecHash {
	templateSpecs, err := k8s.ExtractPodTemplateSpec(e)
	require.NoError(t, err)

	var hashes []k8s.PodTemplateSpecHash
	for _, ts := range templateSpecs {
		h, err := k8s.HashPodTemplateSpec(ts)
		require.NoError(t, err)
		hashes = append(hashes, h)
	}
	return hashes
}

type fixtureSub struct {
	ch chan bool
}

func (s fixtureSub) OnChange(ctx context.Context, st store.RStore) {
	s.ch <- true
}

func (f *testFixture) dispatchDCEvent(m model.Manifest, action dockercompose.Action, containerState dockertypes.ContainerState) {
	f.store.Dispatch(dcwatch.EventAction{
		Event: dockercompose.Event{
			ID:      "fake-container-id",
			Type:    dockercompose.TypeContainer,
			Action:  action,
			Service: m.ManifestName().String(),
		},
		Time:           f.clock.Now(),
		ContainerState: containerState,
	})
}

func deployResultSet(manifest model.Manifest, uid types.UID, hashes []k8s.PodTemplateSpecHash) store.BuildResultSet {
	resultSet := store.BuildResultSet{}
	tag := "deadbeef"
	for _, iTarget := range manifest.ImageTargets {
		localRefTagged := container.MustWithTag(iTarget.Refs.LocalRef(), tag)
		clusterRefTagged := container.MustWithTag(iTarget.Refs.LocalRef(), tag)
		resultSet[iTarget.ID()] = store.NewImageBuildResult(iTarget.ID(), localRefTagged, clusterRefTagged)
	}
	ktID := manifest.K8sTarget().ID()
	resultSet[ktID] = store.NewK8sDeployResult(ktID, []types.UID{uid}, hashes, nil)
	return resultSet
}

func liveUpdateResultSet(manifest model.Manifest, id container.ID) store.BuildResultSet {
	resultSet := store.BuildResultSet{}
	for _, iTarget := range manifest.ImageTargets {
		resultSet[iTarget.ID()] = store.NewLiveUpdateBuildResult(iTarget.ID(), []container.ID{id})
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
	es, err := k8s.ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatalf("error parsing yaml: %v", err)
	}

	if len(es) != 1 {
		t.Fatalf("expected exactly 1 k8s entity from yaml, got %d", len(es))
	}

	e := es[0]
	k8s.SetUIDForTest(t, &e, uid)

	return e
}

type fakeSnapshotUploader struct {
	count int
}

var _ cloud.SnapshotUploader = &fakeSnapshotUploader{}

func (f *fakeSnapshotUploader) TakeAndUpload(state store.EngineState) (cloud.SnapshotID, error) {
	f.count++
	return cloud.SnapshotID(fmt.Sprintf("snapshot%d", f.count)), nil
}

func (f *fakeSnapshotUploader) Upload(token token.Token, teamID string, snapshot *proto_webview.Snapshot) (cloud.SnapshotID, error) {
	panic("not implemented")
}

func (f *fakeSnapshotUploader) IDToSnapshotURL(id cloud.SnapshotID) string {
	panic("not implemented")
}

// stringifyTargetIDs attempts to make a unique string to identify any set of targets
// (order-agnostic) by sorting and then concatenating the target IDs.
func stringifyTargetIDs(targets []model.TargetSpec) string {
	ids := make([]string, len(targets))
	for i, t := range targets {
		ids[i] = t.ID().String()
	}
	sort.Strings(ids)
	return strings.Join(ids, "::")
}

func targetIDStringForManifest(m model.Manifest) string {
	return stringifyTargetIDs(m.TargetSpecs())
}
