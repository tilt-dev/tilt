package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var testImageRef = container.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef")
var imageTargetID = model.TargetID{
	Type: model.TargetTypeImage,
	Name: model.TargetName(apis.SanitizeName("gcr.io/some-project-162817/sancho")),
}

var alreadyBuilt = store.NewImageBuildResultSingleRef(imageTargetID, testImageRef)
var alreadyBuiltSet = store.BuildResultSet{imageTargetID: alreadyBuilt}

type expectedFile = testutils.ExpectedFile

var testPodID k8s.PodID = "pod-id"
var testContainerInfo = liveupdates.Container{
	PodID:         testPodID,
	ContainerID:   k8s.MagicTestContainerID,
	ContainerName: "container-name",
}

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}

	expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	if !strings.Contains(f.k8s.Yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
	}
}

func TestDockerForMacDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	manifest := NewSanchoDockerBuildManifest(f)
	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}

	expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	if !strings.Contains(f.k8s.Yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
	}
}

func TestYamlManifestDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	manifest := manifestbuilder.New(f, "some_yaml").
		WithK8sYAML(testyaml.TracerYAML).Build()
	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestLiveUpdateTaskKilled(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	changed := f.WriteFile("a.txt", "a")

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)
	f.docker.SetExecError(docker.ExitError{ExitCode: build.TaskKillExitCode})

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))

	// Falls back to a build when the exec fails
	assert.Equal(t, 1, f.docker.BuildCount)
}

func TestFallBackToImageDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	f.docker.SetExecError(errors.New("some random error"))

	manifest := NewSanchoLiveUpdateManifest(f)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(intRange{min: 0, max: 0})
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}
}

func TestLiveUpdateFallbackMessagingRedirect(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	syncs := []v1alpha1.LiveUpdateSync{
		{LocalPath: ".", ContainerPath: "/blah"},
	}
	lu := assembleLiveUpdate(syncs,
		nil, false, []string{f.JoinPath("fall_back.txt")}, f)
	manifest := manifestbuilder.New(f, "foobar").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithLiveUpdate(lu).
		WithLiveUpdateBAD().
		WithK8sYAML(SanchoYAML).
		Build()

	changed := f.WriteFile("fall_back.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(intRange{min: 0, max: 0})
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	assert.Contains(t, f.logs.String(), "Will not perform Live Update because",
		"expect logs to contain Live Update-specific fallback message")
}

func TestLiveUpdateFallbackMessagingUnexpectedError(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	f.docker.SetExecError(errors.New("some random error"))

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(intRange{min: 0, max: 0})
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	assert.Contains(t, f.logs.String(), "Live Update failed with unexpected error",
		"expect logs to contain Live Update-specific fallback message")
}

func TestLiveUpdateTwice(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	targets := buildcontrol.BuildTargets(manifest)
	aPath := f.WriteFile("a.txt", "a")
	bPath := f.WriteFile("b.txt", "b")

	firstState := resultToStateSet(manifest, alreadyBuiltSet, []string{aPath}, testContainerInfo)
	firstResult, err := f.BuildAndDeploy(targets, firstState)
	if err != nil {
		t.Fatal(err)
	}

	secondState := resultToStateSet(manifest, firstResult, []string{bPath}, testContainerInfo)
	_, err = f.BuildAndDeploy(targets, secondState)
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 2 {
		t.Errorf("Expected 2 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 2 {
		t.Errorf("Expected 2 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(intRange{min: 2, max: 2})
}

// Kill the pod after the first container update,
// and make sure the next image build gets the right file updates.
func TestLiveUpdateTwiceDeadPod(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	targets := buildcontrol.BuildTargets(manifest)
	aPath := f.WriteFile("a.txt", "a")
	bPath := f.WriteFile("b.txt", "b")

	firstState := resultToStateSet(manifest, alreadyBuiltSet, []string{aPath}, testContainerInfo)
	firstResult, err := f.BuildAndDeploy(targets, firstState)
	if err != nil {
		t.Fatal(err)
	}

	// Kill the pod
	f.docker.SetExecError(fmt.Errorf("Dead pod"))

	secondState := resultToStateSet(manifest, firstResult, []string{bPath}, testContainerInfo)
	_, err = f.BuildAndDeploy(targets, secondState)
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 0 {
		t.Errorf("Expected 0 pushes to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 2 {
		t.Errorf("Expected 2 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 2 {
		t.Errorf("Expected 2 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(intRange{min: 1, max: 1})
}

func TestIgnoredFiles(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	manifest := NewSanchoDockerBuildManifest(f)

	tiltfile := filepath.Join(f.Path(), "Tiltfile")
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithRepos([]model.LocalGitRepo{
		model.LocalGitRepo{
			LocalPath: f.Path(),
		},
	}).WithTiltFilename(tiltfile))

	f.WriteFile("Tiltfile", "# hello world")
	f.WriteFile("a.txt", "a")
	f.WriteFile(".git/index", "garbage")

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(f.docker.BuildContext)
	testutils.AssertFilesInTar(t, tr, []expectedFile{
		expectedFile{
			Path:     "a.txt",
			Contents: "a",
		},
		expectedFile{
			Path:    ".git/index",
			Missing: true,
		},
		expectedFile{
			Path:    "Tiltfile",
			Missing: true,
		},
	})
}

func TestCustomBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifest(f)
	targets := buildcontrol.BuildTargets(manifest)

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}
}

func TestCustomBuildDeterministicTag(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	refStr := "gcr.io/some-project-162817/sancho:deterministic-tag"
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images[refStr] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithTag(f, "deterministic-tag")
	targets := buildcontrol.BuildTargets(manifest)

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}
}

func TestContainerBuildMultiStage(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	manifest := NewSanchoLiveUpdateMultiStageManifest(f)
	targets := buildcontrol.BuildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)

	// There are two image targets. The first has a build result,
	// the second does not --> second target needs build
	iTargetID := targets[0].ID()
	firstResult := store.NewImageBuildResultSingleRef(iTargetID, container.MustParseNamedTagged("sancho-base:tilt-prebuilt"))
	bs[iTargetID] = store.NewBuildState(firstResult, nil, nil)

	result, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	// Docker Build/Push would imply an image build. Make sure they didn't happen,
	// i.e. that we did a LiveUpdate
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)

	// Make sure we did a LiveUpdate (copy files to container, exec in container, restart)
	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))
	f.assertContainerRestarts(intRange{min: 1, max: 1})

	// The BuildComplete action handler expects to get exactly one result
	_, hasResult0 := result[manifest.ImageTargetAt(0).ID()]
	assert.False(t, hasResult0)
	_, hasResult1 := result[manifest.ImageTargetAt(1).ID()]
	assert.True(t, hasResult1)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
}

func TestDockerComposeImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	manifest := NewSanchoLiveUpdateDCManifest(f)
	targets := buildcontrol.BuildTargets(manifest)

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	assert.Empty(t, f.k8s.Yaml, "expect no k8s YAML for DockerCompose resource")
	assert.Len(t, f.dcCli.UpCalls(), 1)
}

func TestDockerComposeLiveUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeContainerd)

	manifest := NewSanchoLiveUpdateDCManifest(f)
	targets := buildcontrol.BuildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)

	_, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))
	assert.Empty(t, f.k8s.Yaml, "expect no k8s YAML for DockerCompose resource")
	assert.Empty(t, 0, f.k8s.ExecCalls,
		"Expected no k8s Exec calls, actual: %d", f.k8s.ExecCalls)
	f.assertContainerRestarts(intRange{min: 1, max: 1})
}

func TestReturnLastUnexpectedError(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoLiveUpdateManifest(f)
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}
}

// errors get logged by the upper, so make sure our builder isn't logging the error redundantly
func TestDockerBuildErrorNotLogged(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoDockerBuildManifest(f)
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}

	logs := f.logs.String()
	require.Equal(t, 0, strings.Count(logs, "no one expects the unexpected error"))
}

func TestLiveUpdateWithRunFailureReturnsContainerIDs(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	// LiveUpdate will failure with a RunStepFailure
	f.docker.SetExecError(userFailureErrDocker)

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	targets := buildcontrol.BuildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed}, testContainerInfo)
	resultSet, err := f.BuildAndDeploy(targets, bs)
	require.NotNil(t, err, "expected failed LiveUpdate to return error")

	iTargID := manifest.ImageTargetAt(0).ID()
	result := resultSet[iTargID]
	res, ok := result.(store.LiveUpdateBuildResult)
	require.True(t, ok, "expected build result for image target %s", iTargID)
	require.Len(t, res.LiveUpdatedContainerIDs, 1)
	require.Equal(t, res.LiveUpdatedContainerIDs[0].String(), k8s.MagicTestContainerID)

	// LiveUpdate failed due to RunStepError, should NOT fall back to image build
	assert.Equal(t, 0, f.docker.BuildCount, "expect no image build -> no docker build calls")
	f.assertK8sUpsertCalled(false)

	// Copied files and tried to docker.exec before hitting error
	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))
}

func TestLiveUpdateMultipleImagesSamePod(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// expect live update and NOT an image build
	require.Equal(t, 0, f.docker.BuildCount)
	require.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(false)

	// (1 x sync / run / restart) x 2 containers
	require.Equal(t, 2, f.docker.CopyCount)
	require.Equal(t, 2, len(f.docker.ExecCalls))
	require.Equal(t, 2, len(f.docker.RestartsByContainer))
	for k, v := range f.docker.RestartsByContainer {
		assert.Equal(t, 1, v, "# restarts for container %q", k)
	}

}

func TestOneLiveUpdateOneDockerBuildDoesImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	sanchoTarg := NewSanchoLiveUpdateImageTarget(f)          // first target = LiveUpdate
	sidecarTarg := NewSanchoSidecarDockerBuildImageTarget(f) // second target = DockerBuild
	sanchoRef := container.MustParseNamedTagged(fmt.Sprintf("%s:tilt-123", testyaml.SanchoImage))
	sidecarRef := container.MustParseNamedTagged(fmt.Sprintf("%s:tilt-123", testyaml.SanchoSidecarImage))
	sanchoCInfo := liveupdates.Container{
		PodID:         testPodID,
		ContainerName: "sancho",
		ContainerID:   "sancho-c",
	}

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoSidecarYAML).
		WithImageTargets(sanchoTarg, sidecarTarg).
		Build()
	changed := f.WriteFile("a.txt", "a")
	sanchoState := liveupdates.WithFakeK8sContainers(
		store.NewBuildState(store.NewImageBuildResultSingleRef(sanchoTarg.ID(), sanchoRef), []string{changed}, nil),
		sanchoRef.String(), []liveupdates.Container{sanchoCInfo})
	sidecarState := store.NewBuildState(store.NewImageBuildResultSingleRef(sidecarTarg.ID(), sidecarRef), []string{changed}, nil)

	bs := store.BuildStateSet{sanchoTarg.ID(): sanchoState, sidecarTarg.ID(): sidecarState}

	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// expect an image build
	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 2, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestLiveUpdateMultipleImagesOneRunErrorExecutesRestOfLiveUpdatesAndDoesntImageBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): fix this")
	}
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	// First LiveUpdate will simulate a failed Run step
	f.docker.ExecErrorsToThrow = []error{userFailureErrDocker}

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	result, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), bs)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Run step \"go install github.com/tilt-dev/sancho\" failed with exit code: 123")

	// one for each container update
	assert.Equal(t, 2, f.docker.CopyCount)
	assert.Equal(t, 2, len(f.docker.ExecCalls))

	// expect NO image build
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(false)

	// Make sure we returned the CIDs we LiveUpdated --
	// they contain state now, we'll want to track them
	liveUpdatedCIDs := result.LiveUpdatedContainerIDs()
	expectedCIDs := []container.ID{"sancho-c", "sidecar-c"}
	assert.ElementsMatch(t, expectedCIDs, liveUpdatedCIDs)
}

func TestLiveUpdateMultipleImagesOneUpdateErrorFallsBackToImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	// Second LiveUpdate will throw an error
	f.docker.ExecErrorsToThrow = []error{nil, fmt.Errorf("whelp ¯\\_(ツ)_/¯")}

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// one for each container update
	assert.Equal(t, 2, f.docker.CopyCount)
	assert.Equal(t, 2, len(f.docker.ExecCalls)) // second one errors

	// expect image build (2x images) when we fall back from failed LiveUpdate
	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestLiveUpdateMultipleImagesOneWithUnsyncedChangeFileFallsBackToImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	bs[manifest.ImageTargetAt(1).ID()].FilesChangedSet["/not/synced"] = true // changed file not in a sync --> fall back to image build
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// expect image build (2x images) when we fall back from failed LiveUpdate
	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 2, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestLocalTargetDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	lt := model.NewLocalTarget("hello-world", model.ToHostCmd("echo hello world"), model.Cmd{}, nil)
	res, err := f.BuildAndDeploy([]model.TargetSpec{lt}, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, 0, f.docker.BuildCount, "should have 0 docker builds")
	assert.Equal(t, 0, f.docker.PushCount, "should have 0 docker pushes")
	assert.Empty(t, f.k8s.Yaml, "should not apply any k8s yaml")
	assert.Len(t, res, 1, "expect exactly one result in result set")
	assert.Contains(t, f.logs.String(), "hello world", "logs should contain cmd output")
}

func TestLocalTargetFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	lt := model.NewLocalTarget("hello-world", model.ToHostCmd("echo 'oh no' && exit 1"), model.Cmd{}, nil)
	res, err := f.BuildAndDeploy([]model.TargetSpec{lt}, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty result for failed command")

	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "exit status 1", "error msg should indicate command failure")
	assert.Contains(t, f.logs.String(), "oh no", "logs should contain cmd output")

	assert.Equal(t, 0, f.docker.BuildCount, "should have 0 docker builds")
	assert.Equal(t, 0, f.docker.PushCount, "should have 0 docker pushes")
	assert.Empty(t, f.k8s.Yaml, "should not apply any k8s yaml")
}

func multiImageLiveUpdateManifestAndBuildState(f *bdFixture) (model.Manifest, store.BuildStateSet) {
	sanchoTarg := NewSanchoLiveUpdateImageTarget(f)
	sidecarTarg := NewSanchoSidecarLiveUpdateImageTarget(f)
	sanchoRef := container.MustParseNamedTagged(fmt.Sprintf("%s:tilt-123", testyaml.SanchoImage))
	sidecarRef := container.MustParseNamedTagged(fmt.Sprintf("%s:tilt-123", testyaml.SanchoSidecarImage))
	sanchoCInfo := liveupdates.Container{
		PodID:         testPodID,
		ContainerName: "sancho",
		ContainerID:   "sancho-c",
	}
	sidecarCInfo := liveupdates.Container{
		PodID:         testPodID,
		ContainerName: "sancho-sidecar",
		ContainerID:   "sidecar-c",
	}

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoSidecarYAML).
		WithImageTargets(sanchoTarg, sidecarTarg).
		WithLiveUpdateBAD().
		Build()

	changed := f.WriteFile("a.txt", "a")
	sanchoState := liveupdates.WithFakeK8sContainers(
		store.NewBuildState(store.NewImageBuildResultSingleRef(sanchoTarg.ID(), sanchoRef), []string{changed}, nil),
		string(sanchoTarg.ID().Name), []liveupdates.Container{sanchoCInfo})
	sidecarState := liveupdates.WithFakeK8sContainers(
		store.NewBuildState(store.NewImageBuildResultSingleRef(sidecarTarg.ID(), sidecarRef), []string{changed}, nil),
		string(sidecarTarg.ID().Name), []liveupdates.Container{sidecarCInfo})

	bs := store.BuildStateSet{sanchoTarg.ID(): sanchoState, sidecarTarg.ID(): sidecarState}

	return manifest, bs
}

type testStore struct {
	*store.TestingStore
	out io.Writer
}

func NewTestingStore(out io.Writer) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	if action, ok := action.(store.LogAction); ok {
		_, _ = s.out.Write(action.Message())
	}
}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake Client and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	cancel     func()
	docker     *docker.FakeClient
	k8s        *k8s.FakeK8sClient
	bd         buildcontrol.BuildAndDeployer
	st         *testStore
	dcCli      *dockercompose.FakeDCClient
	logs       *bytes.Buffer
	ctrlClient ctrlclient.Client
}

func newBDFixture(t *testing.T, env k8s.Env, runtime container.Runtime) *bdFixture {
	return newBDFixtureWithUpdateMode(t, env, runtime, liveupdates.UpdateModeAuto)
}

func newBDFixtureWithUpdateMode(t *testing.T, env k8s.Env, runtime container.Runtime, um liveupdates.UpdateMode) *bdFixture {
	logs := new(bytes.Buffer)
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsForTest(logs)
	ctx, cancel := context.WithCancel(ctx)
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewTiltDevDirAt(f.Path())
	dockerClient := docker.NewFakeClient()
	dockerClient.ContainerListOutput = map[string][]types.Container{
		"pod": []types.Container{
			types.Container{
				ID: k8s.MagicTestContainerID,
			},
		},
	}
	k8s := k8s.NewFakeK8sClient(t)
	k8s.Runtime = runtime
	mode := liveupdates.UpdateModeFlag(um)
	dcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	kl := &fakeKINDLoader{}
	ctrlClient := fake.NewFakeTiltClient()
	st := NewTestingStore(logs)
	execer := localexec.NewFakeExecer(t)
	bd, err := provideFakeBuildAndDeployer(ctx, dockerClient, k8s, dir, env, mode, dcc,
		fakeClock{now: time.Unix(1551202573, 0)}, kl, ta, ctrlClient, st, execer)
	require.NoError(t, err)

	ret := &bdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		docker:         dockerClient,
		k8s:            k8s,
		bd:             bd,
		st:             st,
		dcCli:          dcc,
		logs:           logs,
		ctrlClient:     ctrlClient,
	}

	t.Cleanup(ret.TearDown)
	return ret
}

func (f *bdFixture) TearDown() {
	f.cancel()
}

func (f *bdFixture) NewPathSet(paths ...string) model.PathSet {
	return model.NewPathSet(paths, f.Path())
}

func (f *bdFixture) assertContainerRestarts(ir intRange) {
	// Ensure that MagicTestContainerID was the only container id that saw
	// restarts, and that it saw the right number of restarts.
	min := map[string]int{}
	max := map[string]int{}
	if ir.min != 0 {
		min[string(k8s.MagicTestContainerID)] = ir.min
		max[string(k8s.MagicTestContainerID)] = ir.max
	}
	assert.GreaterOrEqual(f.T(), min, f.docker.RestartsByContainer,
		"checking for expected # of container restarts")
	assert.LessOrEqual(f.T(), max, f.docker.RestartsByContainer,
		"checking for expected # of container restarts")
}

// Total number of restarts, regardless of which container.
func (f *bdFixture) assertTotalContainerRestarts(ir intRange) {
	assert.GreaterOrEqual(f.T(), len(f.docker.RestartsByContainer), ir.min,
		"checking for expected # of container restarts")
	assert.GreaterOrEqual(f.T(), len(f.docker.RestartsByContainer), ir.max,
		"checking for expected # of container restarts")
}

func (f *bdFixture) assertK8sUpsertCalled(called bool) {
	assert.Equal(f.T(), called, f.k8s.Yaml != "",
		"checking that k8s.Upsert was called")
}

func (f *bdFixture) upsert(obj ctrlclient.Object) {
	require.True(f.T(), obj.GetName() != "",
		"object has empty name")

	err := f.ctrlClient.Create(f.ctx, obj)
	if err == nil {
		return
	}

	copy := obj.DeepCopyObject().(ctrlclient.Object)
	err = f.ctrlClient.Get(f.ctx, ktypes.NamespacedName{Name: obj.GetName()}, copy)
	assert.NoError(f.T(), err)

	obj.SetResourceVersion(copy.GetResourceVersion())

	err = f.ctrlClient.Update(f.ctx, obj)
	assert.NoError(f.T(), err)
}

func (f *bdFixture) BuildAndDeploy(specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	for _, spec := range specs {
		localTarget, ok := spec.(model.LocalTarget)
		if ok && localTarget.UpdateCmdSpec != nil {
			cmd := v1alpha1.Cmd{
				ObjectMeta: metav1.ObjectMeta{Name: localTarget.UpdateCmdName()},
				Spec:       *(localTarget.UpdateCmdSpec),
			}
			f.upsert(&cmd)
		}

		iTarget, ok := spec.(model.ImageTarget)
		if ok {
			im := v1alpha1.ImageMap{
				ObjectMeta: metav1.ObjectMeta{Name: iTarget.ID().Name.String()},
				Spec:       iTarget.ImageMapSpec,
			}
			state := stateSet[iTarget.ID()]
			imageBuildResult, ok := state.LastResult.(store.ImageBuildResult)
			if ok {
				im.Status = imageBuildResult.ImageMapStatus
			}
			f.upsert(&im)

			if !liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec) {
				lu := v1alpha1.LiveUpdate{
					ObjectMeta: metav1.ObjectMeta{Name: iTarget.LiveUpdateName},
					Spec:       iTarget.LiveUpdateSpec,
				}
				f.upsert(&lu)
			}

			if iTarget.IsDockerBuild() {
				di := v1alpha1.DockerImage{
					ObjectMeta: metav1.ObjectMeta{Name: iTarget.DockerImageName},
					Spec:       iTarget.DockerBuildInfo().DockerImageSpec,
				}
				f.upsert(&di)
			}
		}

		kTarget, ok := spec.(model.K8sTarget)
		if ok {
			ka := v1alpha1.KubernetesApply{
				ObjectMeta: metav1.ObjectMeta{Name: kTarget.ID().Name.String()},
				Spec:       kTarget.KubernetesApplySpec,
			}
			f.upsert(&ka)
		}
	}
	return f.bd.BuildAndDeploy(f.ctx, f.st, specs, stateSet)
}

func (f *bdFixture) createBuildStateSet(manifest model.Manifest, changedFiles []string) store.BuildStateSet {
	bs := store.BuildStateSet{}

	// If there are no changed files, the test wants a build state where
	// nothing has ever been built.
	//
	// If there are changed files, the test wants a build state where
	// everything has been built once. The changed files chould be
	// attached to the appropriate build state.
	if len(changedFiles) == 0 {
		return bs
	}

	consumedFiles := make(map[string]bool)
	for _, iTarget := range manifest.ImageTargets {
		filesChangingImage := []string{}
		for _, file := range changedFiles {
			fullPath := f.JoinPath(file)
			inDeps := false
			for _, dep := range iTarget.Dependencies() {
				if ospath.IsChild(dep, fullPath) {
					inDeps = true
					break
				}
			}

			if inDeps {
				filesChangingImage = append(filesChangingImage, f.WriteFile(file, "blah"))
				consumedFiles[file] = true
			}
		}

		state := store.NewBuildState(alreadyBuilt, filesChangingImage, nil)
		if manifest.IsImageDeployed(iTarget) {
			if manifest.IsDC() {
				state = liveupdates.WithFakeDCContainer(state, testContainerInfo)
			} else {
				state = liveupdates.WithFakeK8sContainers(state, string(iTarget.ID().Name), []liveupdates.Container{testContainerInfo})
			}
		}
		bs[iTarget.ID()] = state
	}

	if len(consumedFiles) != len(changedFiles) {
		f.T().Fatalf("testCase has files that weren't consumed by an image. "+
			"Was that intentional?\nChangedFiles: %v\nConsumedFiles: %v\n",
			changedFiles, consumedFiles)
	}
	return bs
}

func resultToStateSet(m model.Manifest, resultSet store.BuildResultSet, files []string, container liveupdates.Container) store.BuildStateSet {
	stateSet := store.BuildStateSet{}
	for id, result := range resultSet {
		state := store.NewBuildState(result, files, nil)
		if m.IsDC() {
			state = liveupdates.WithFakeDCContainer(state, container)
		} else {
			state = liveupdates.WithFakeK8sContainers(state, string(id.Name), []liveupdates.Container{container})
		}
		stateSet[id] = state
	}
	return stateSet
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

type fakeKINDLoader struct {
	loadCount int
}

func (kl *fakeKINDLoader) LoadToKIND(ctx context.Context, ref reference.NamedTagged) error {
	kl.loadCount++
	return nil
}
