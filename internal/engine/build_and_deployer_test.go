package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/synclet"
	"github.com/tilt-dev/tilt/internal/synclet/sidecar"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

var testImageRef = container.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef")
var imageTargetID = model.TargetID{
	Type: model.TargetTypeImage,
	Name: "gcr.io/some-project-162817/sancho",
}

var alreadyBuilt = store.NewImageBuildResultSingleRef(imageTargetID, testImageRef)
var alreadyBuiltSet = store.BuildResultSet{imageTargetID: alreadyBuilt}

type expectedFile = testutils.ExpectedFile

var testPodID k8s.PodID = "pod-id"
var testContainerInfo = store.ContainerInfo{
	PodID:         testPodID,
	ContainerID:   k8s.MagicTestContainerID,
	ContainerName: "container-name",
}

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
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

	if !strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should deploy the synclet on docker-for-desktop: %s", f.k8s.Yaml)
	}
}

func TestDockerForMacDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
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

	if strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should not deploy the synclet on docker-for-desktop: %s", f.k8s.Yaml)
	}
}

func TestSyncletNamespaceGKE(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	assert.Equal(t, "", string(f.sCli.Namespace))
	assert.Equal(t, "", string(f.k8s.LastPodQueryNamespace))

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	cInfo := testContainerInfo
	cInfo.Namespace = "sancho-ns"

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(result, []string{changed}, cInfo)
	_, err = f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "sancho-ns", string(f.sCli.Namespace))
}

func TestYamlManifestDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest := manifestbuilder.New(f, "some_yaml").
		WithK8sYAML(testyaml.TracerYAML).Build()
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.sCli.UpdateContainerCount)
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestContainerBuildLocal(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 1 {
		t.Errorf("Expected 1 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 1 {
		t.Errorf("Expected 1 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(1)

	id := manifest.ImageTargetAt(0).ID()
	_, hasResult := result[id]
	assert.True(t, hasResult)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
}

func TestContainerBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount,
		"Expected no docker build, actual: %d", f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount,
		"Expected no push to docker, actual: %d", f.docker.PushCount)
	assert.Equal(t, 1, f.sCli.UpdateContainerCount,
		"Expected 1 synclet UpdateContainer call, actual: %d", f.sCli.UpdateContainerCount)
	assert.Equal(t, 1, f.docker.CopyCount,
		"Expected 1 docker CopyToContainer (via synclet), actual: %d", f.docker.CopyCount)
	assert.Len(t, f.docker.ExecCalls, 1,
		"Expected 1 docker Exec call (via synclet), actual: %d", len(f.docker.ExecCalls))
	f.assertContainerRestarts(1)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
}

func TestContainerBuildLocalTriggeredRuns(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifestWithTriggeredRuns(f, true)
	changed := f.WriteFile("a.txt", "a") // matches one run trigger, not the other

	targets := buildTargets(manifest)
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 1 {
		t.Errorf("Expected 1 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 2 {
		t.Errorf("Expected 2 exec in container calls, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(1)

	id := manifest.ImageTargetAt(0).ID()
	_, hasResult := result[id]
	assert.True(t, hasResult)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
}

func TestContainerBuildSyncletTriggeredRuns(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifestWithTriggeredRuns(f, true)
	changed := f.WriteFile("a.txt", "a") // matches one run trigger, not the other

	targets := buildTargets(manifest)
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount, "Expected no docker build, actual: %d", f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount, "Expected no push to docker, actual: %d", f.docker.PushCount)
	assert.Equal(t, 1, f.sCli.UpdateContainerCount,
		"Expected 1 synclet UpdateContainer call, actual: %d", f.sCli.UpdateContainerCount)
	assert.Equal(t, 1, f.docker.CopyCount,
		"Expected 1 docker CopyToContainer (via synclet), actual: %d", f.docker.CopyCount)
	assert.Len(t, f.docker.ExecCalls, 2,
		"Expected 1 docker Exec call (via synclet), actual: %d", len(f.docker.ExecCalls))
	f.assertContainerRestarts(1)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
}

func TestContainerBuildSyncletHotReload(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifestWithTriggeredRuns(f, false)
	changed := f.WriteFile("a.txt", "a") // matches one run trigger, not the other
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, f.sCli.UpdateContainerCount,
		"Expected 1 synclet UpdateContainer call, actual: %d", f.sCli.UpdateContainerCount)
	f.assertContainerRestarts(0)
}

func TestLiveUpdateFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	f.docker.SetExecError(docker.ExitError{ExitCode: 1})

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	msg := "Run step \"go install github.com/tilt-dev/sancho\" failed with exit code: 1"
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expected error message %q, actual: %v", msg, err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 1 {
		t.Errorf("Expected 1 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 1 {
		t.Errorf("Expected 1 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(0)
}

func TestLiveUpdateTaskKilled(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")

	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	f.docker.SetExecError(docker.ExitError{ExitCode: build.TaskKillExitCode})

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
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
	defer f.TearDown()

	f.docker.SetExecError(errors.New("some random error"))

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(0)
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}
}

func TestNoFallbackForDontFallBackError(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()
	f.docker.SetExecError(buildcontrol.DontFallBackErrorf("i'm melllting"))

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err == nil {
		t.Errorf("Expected this error to fail fallback tester and propagate back up")
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
}

func TestLiveUpdateFallbackMessagingRedirect(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	lu := assembleLiveUpdate([]model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{Source: f.Path(), Dest: "/blah"}},
		nil, false, []string{f.JoinPath("fall_back.txt")}, f)
	manifest := manifestbuilder.New(f, "foobar").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithLiveUpdate(lu).
		WithK8sYAML(SanchoYAML).
		Build()

	changed := f.WriteFile("fall_back.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(0)
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	assert.Contains(t, f.logs.String(), "Will not perform Live Update because",
		"expect logs to contain Live Update-specific fallback message")
}

func TestLiveUpdateFallbackMessagingUnexpectedError(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	f.docker.SetExecError(errors.New("some random error"))

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(0)
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	assert.Contains(t, f.logs.String(), "Live Update failed with unexpected error",
		"expect logs to contain Live Update-specific fallback message")
}

func TestLiveUpdateTwice(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	aPath := f.WriteFile("a.txt", "a")
	bPath := f.WriteFile("b.txt", "b")

	firstState := resultToStateSet(alreadyBuiltSet, []string{aPath}, testContainerInfo)
	firstResult, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, firstState)
	if err != nil {
		t.Fatal(err)
	}

	secondState := resultToStateSet(firstResult, []string{bPath}, testContainerInfo)
	_, err = f.bd.BuildAndDeploy(f.ctx, f.st, targets, secondState)
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
	f.assertContainerRestarts(2)
}

// Kill the pod after the first container update,
// and make sure the next image build gets the right file updates.
func TestLiveUpdateTwiceDeadPod(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	aPath := f.WriteFile("a.txt", "a")
	bPath := f.WriteFile("b.txt", "b")

	firstState := resultToStateSet(alreadyBuiltSet, []string{aPath}, testContainerInfo)
	firstResult, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, firstState)
	if err != nil {
		t.Fatal(err)
	}

	// Kill the pod
	f.docker.SetExecError(fmt.Errorf("Dead pod"))

	secondState := resultToStateSet(firstResult, []string{bPath}, testContainerInfo)
	_, err = f.bd.BuildAndDeploy(f.ctx, f.st, targets, secondState)
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
	f.assertContainerRestarts(1)
}

func TestIgnoredFiles(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

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

	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(f.docker.BuildOptions.Context)
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

func TestCustomBuildWithLiveUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithLiveUpdate(f)
	targets := buildTargets(manifest)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}

	if !strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should deploy the synclet for a custom build with a live update: %s", f.k8s.Yaml)
	}
}

func TestCustomBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifest(f)
	targets := buildTargets(manifest)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}

	if strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should not deploy the synclet for a custom build: %s", f.k8s.Yaml)
	}
}

func TestCustomBuildDeterministicTag(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()
	refStr := "gcr.io/some-project-162817/sancho:deterministic-tag"
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images[refStr] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithTag(f, "deterministic-tag")
	targets := buildTargets(manifest)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}

	if strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should not deploy the synclet for a custom build: %s", f.k8s.Yaml)
	}
}

func TestContainerBuildMultiStage(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateMultiStageManifest(f)
	targets := buildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	// There are two image targets. The first has a build result,
	// the second does not --> second target needs build
	iTargetID := targets[0].ID()
	firstResult := store.NewImageBuildResultSingleRef(iTargetID, container.MustParseNamedTagged("sancho-base:tilt-prebuilt"))
	bs[iTargetID] = store.NewBuildState(firstResult, nil, nil)

	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
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
	f.assertContainerRestarts(1)

	// The BuildComplete action handler expects to get exactly one result
	_, hasResult0 := result[manifest.ImageTargetAt(0).ID()]
	assert.False(t, hasResult0)
	_, hasResult1 := result[manifest.ImageTargetAt(1).ID()]
	assert.True(t, hasResult1)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
}

func TestDockerComposeImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateDCManifest(f)
	targets := buildTargets(manifest)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	if strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should not deploy the synclet for a docker-compose build: %s", f.k8s.Yaml)
	}
	assert.Len(t, f.dcCli.UpCalls, 1)
}

func TestDockerComposeLiveUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateDCManifest(f)
	targets := buildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))
	assert.Equal(t, 0, f.sCli.UpdateContainerCount,
		"Expected no synclet UpdateContainer call, actual: %d", f.sCli.UpdateContainerCount)
	if strings.Contains(f.k8s.Yaml, sidecar.DefaultSyncletImageName) {
		t.Errorf("Should not deploy the synclet for a docker-compose build: %s", f.k8s.Yaml)
	}
	f.assertContainerRestarts(1)
}

func TestReturnLastUnexpectedError(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoLiveUpdateManifest(f)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}
}

// errors get logged by the upper, so make sure our builder isn't logging the error redundantly
func TestDockerBuildErrorNotLogged(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoDockerBuildManifest(f)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}

	logs := f.logs.String()
	require.Equal(t, 0, strings.Count(logs, "no one expects the unexpected error"))
}

func TestLiveUpdateWithRunFailureReturnsContainerIDs(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	// LiveUpdate will failure with a RunStepFailure
	f.docker.SetExecError(userFailureErrDocker)

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, testContainerInfo)
	resultSet, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
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
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// expect live update and NOT an image build
	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(false)

	// one for each container update
	assert.Equal(t, 2, f.sCli.UpdateContainerCount)
}

func TestOneLiveUpdateOneDockerBuildDoesImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	sanchoTarg := NewSanchoLiveUpdateImageTarget(f)          // first target = LiveUpdate
	sidecarTarg := NewSanchoSidecarDockerBuildImageTarget(f) // second target = DockerBuild
	sanchoRef := container.MustParseNamedTagged(fmt.Sprintf("%s:tilt-123", testyaml.SanchoImage))
	sidecarRef := container.MustParseNamedTagged(fmt.Sprintf("%s:tilt-123", testyaml.SanchoSidecarImage))
	sanchoCInfo := store.ContainerInfo{
		PodID:         testPodID,
		ContainerName: "sancho",
		ContainerID:   "sancho-c",
	}

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoSidecarYAML).
		WithImageTargets(sanchoTarg, sidecarTarg).
		Build()
	changed := f.WriteFile("a.txt", "a")
	sanchoState := store.NewBuildState(store.NewImageBuildResultSingleRef(sanchoTarg.ID(), sanchoRef), []string{changed}, nil).
		WithRunningContainers([]store.ContainerInfo{sanchoCInfo})
	sidecarState := store.NewBuildState(store.NewImageBuildResultSingleRef(sidecarTarg.ID(), sidecarRef), []string{changed}, nil)

	bs := store.BuildStateSet{sanchoTarg.ID(): sanchoState, sidecarTarg.ID(): sidecarState}

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// expect an image build
	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 2, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)

	// should NOT have run live update
	assert.Equal(t, 0, f.sCli.UpdateContainerCount)
}

func TestLiveUpdateMultipleImagesOneRunErrorExecutesRestOfLiveUpdatesAndDoesntImageBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): fix this")
	}
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	// First LiveUpdate will simulate a failed Run step
	f.docker.ExecErrorsToThrow = []error{userFailureErrDocker}

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
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
	defer f.TearDown()

	// Second LiveUpdate will throw an error
	f.docker.ExecErrorsToThrow = []error{nil, fmt.Errorf("whelp ¯\\_(ツ)_/¯")}

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
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
	defer f.TearDown()

	manifest, bs := multiImageLiveUpdateManifestAndBuildState(f)
	bs[manifest.ImageTargetAt(1).ID()].FilesChangedSet["/not/synced"] = true // changed file not in a sync --> fall back to image build
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	// should NOT have run live update
	assert.Equal(t, 0, f.sCli.UpdateContainerCount)

	// expect image build (2x images) when we fall back from failed LiveUpdate
	assert.Equal(t, 2, f.docker.BuildCount)
	assert.Equal(t, 2, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestLocalTargetDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lt := model.LocalTarget{UpdateCmd: model.ToHostCmd("echo hello world")}
	res, err := f.bd.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{lt}, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, 0, f.docker.BuildCount, "should have 0 docker builds")
	assert.Equal(t, 0, f.docker.PushCount, "should have 0 docker pushes")
	assert.Empty(t, f.k8s.Yaml, "should not apply any k8s yaml")
	assert.Len(t, res, 1, "expect exactly one result in result set")
	assert.Contains(t, f.logs.String(), "hello world", "logs should contain cmd output")
}

func TestLocalTargetFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lt := model.LocalTarget{UpdateCmd: model.ToHostCmd("echo 'oh no' && exit 1")}
	res, err := f.bd.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{lt}, store.BuildStateSet{})
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
	sanchoCInfo := store.ContainerInfo{
		PodID:         testPodID,
		ContainerName: "sancho",
		ContainerID:   "sancho-c",
	}
	sidecarCInfo := store.ContainerInfo{
		PodID:         testPodID,
		ContainerName: "sancho-sidecar",
		ContainerID:   "sidecar-c",
	}

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoSidecarYAML).
		WithImageTargets(sanchoTarg, sidecarTarg).
		Build()

	changed := f.WriteFile("a.txt", "a")
	sanchoState := store.NewBuildState(store.NewImageBuildResultSingleRef(sanchoTarg.ID(), sanchoRef), []string{changed}, nil).
		WithRunningContainers([]store.ContainerInfo{sanchoCInfo})
	sidecarState := store.NewBuildState(store.NewImageBuildResultSingleRef(sidecarTarg.ID(), sidecarRef), []string{changed}, nil).
		WithRunningContainers([]store.ContainerInfo{sidecarCInfo})

	bs := store.BuildStateSet{sanchoTarg.ID(): sanchoState, sidecarTarg.ID(): sidecarState}

	return manifest, bs
}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake Client and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	docker *docker.FakeClient
	k8s    *k8s.FakeK8sClient
	sCli   *synclet.TestSyncletClient
	bd     BuildAndDeployer
	st     *store.TestingStore
	dcCli  *dockercompose.FakeDCClient
	logs   *bytes.Buffer
}

func newBDFixture(t *testing.T, env k8s.Env, runtime container.Runtime) *bdFixture {
	logs := new(bytes.Buffer)
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsForTest(logs)
	ctx, cancel := context.WithCancel(ctx)
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(f.Path())
	docker := docker.NewFakeClient()
	docker.ContainerListOutput = map[string][]types.Container{
		"pod": []types.Container{
			types.Container{
				ID: k8s.MagicTestContainerID,
			},
		},
	}
	k8s := k8s.NewFakeK8sClient()
	k8s.Runtime = runtime
	sCli := synclet.NewTestSyncletClient(docker)
	mode := buildcontrol.UpdateModeFlag(buildcontrol.UpdateModeAuto)
	dcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	kl := &fakeKINDLoader{}
	bd, err := provideBuildAndDeployer(ctx, docker, k8s, dir, env, mode, sCli, dcc, fakeClock{now: time.Unix(1551202573, 0)}, kl, ta)
	if err != nil {
		t.Fatal(err)
	}

	st := store.NewTestingStore()

	return &bdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		docker:         docker,
		k8s:            k8s,
		sCli:           sCli,
		bd:             bd,
		st:             st,
		dcCli:          dcc,
		logs:           logs,
	}
}

func (f *bdFixture) TearDown() {
	f.k8s.TearDown()
	f.cancel()
	f.TempDirFixture.TearDown()
}

func (f *bdFixture) NewPathSet(paths ...string) model.PathSet {
	return model.NewPathSet(paths, f.Path())
}

func (f *bdFixture) assertContainerRestarts(count int) {
	// Ensure that MagicTestContainerID was the only container id that saw
	// restarts, and that it saw the right number of restarts.
	expected := map[string]int{}
	if count != 0 {
		expected[string(k8s.MagicTestContainerID)] = count
	}
	assert.Equal(f.T(), expected, f.docker.RestartsByContainer,
		"checking for expected # of container restarts")
}

// Total number of restarts, regardless of which container.
func (f *bdFixture) assertTotalContainerRestarts(count int) {
	assert.Len(f.T(), f.docker.RestartsByContainer, count,
		"checking for expected # of container restarts")
}

func (f *bdFixture) assertK8sUpsertCalled(called bool) {
	assert.Equal(f.T(), called, f.k8s.Yaml != "",
		"checking that k8s.Upsert was called")
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
			state = state.WithRunningContainers([]store.ContainerInfo{testContainerInfo})
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

func resultToStateSet(resultSet store.BuildResultSet, files []string, cInfo store.ContainerInfo) store.BuildStateSet {
	stateSet := store.BuildStateSet{}
	for id, result := range resultSet {
		state := store.NewBuildState(result, files, nil).WithRunningContainers([]store.ContainerInfo{cInfo})
		stateSet[id] = state
	}
	return stateSet
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }
