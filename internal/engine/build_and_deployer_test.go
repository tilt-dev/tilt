package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

var testImageRef = container.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef")
var imageTargetID = model.TargetID{
	Type: model.TargetTypeImage,
	Name: "gcr.io/some-project-162817/sancho",
}

var alreadyBuilt = store.NewImageBuildResult(imageTargetID, testImageRef)
var alreadyBuiltSet = store.BuildResultSet{imageTargetID: alreadyBuilt}

type expectedFile = testutils.ExpectedFile

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoFastBuildManifest(f)
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

	if !strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should deploy the synclet on docker-for-desktop: %s", f.k8s.Yaml)
	}
}

func TestDockerForMacDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	manifest := NewSanchoFastBuildManifest(f)
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

	if strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should not deploy the synclet on docker-for-desktop: %s", f.k8s.Yaml)
	}
}

func TestNamespaceGKE(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	assert.Equal(t, "", string(f.sCli.Namespace))
	assert.Equal(t, "", string(f.k8s.LastPodQueryNamespace))

	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	deployInfo := f.deployInfo()
	deployInfo.Namespace = "sancho-ns"

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(result, []string{changed}, deployInfo)
	result, err = f.bd.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "sancho-ns", string(f.sCli.Namespace))
}

func TestContainerBuildLocal(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
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
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
}

func TestContainerBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
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
	if f.sCli.UpdateContainerCount != 1 {
		t.Errorf("Expected 1 synclet containerUpdate, actual: %d", f.sCli.UpdateContainerCount)
	}

	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
	assert.False(t, f.sCli.UpdateContainerHotReload)
}

func TestContainerBuildLocalTriggeredRuns(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	manifest := NewSanchoFastBuildManifest(f)
	iTarg := manifest.ImageTargetAt(0)
	fb := iTarg.FastBuildInfo()
	runs := []model.Run{
		model.Run{Cmd: model.ToShellCmd("echo hello")},
		model.Run{Cmd: model.ToShellCmd("echo a"), Triggers: f.NewPathSet("a.txt")}, // matches changed file
		model.Run{Cmd: model.ToShellCmd("echo b"), Triggers: f.NewPathSet("b.txt")}, // does NOT match changed file
	}
	fb.Runs = runs
	iTarg = iTarg.WithBuildDetails(fb)
	manifest = manifest.WithImageTarget(iTarg)

	targets := buildTargets(manifest)
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
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
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
}

func TestContainerBuildSyncletTriggeredRuns(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	manifest := NewSanchoFastBuildManifest(f)
	iTarg := manifest.ImageTargetAt(0)
	fb := iTarg.FastBuildInfo()
	runs := []model.Run{
		model.Run{Cmd: model.ToShellCmd("echo hello")},
		model.Run{Cmd: model.ToShellCmd("echo a"), Triggers: f.NewPathSet("a.txt")}, // matches changed file
		model.Run{Cmd: model.ToShellCmd("echo b"), Triggers: f.NewPathSet("b.txt")}, // does NOT match changed file
	}
	fb.Runs = runs
	iTarg = iTarg.WithBuildDetails(fb)
	manifest = manifest.WithImageTarget(iTarg)

	targets := buildTargets(manifest)
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
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
	if f.sCli.UpdateContainerCount != 1 {
		t.Errorf("Expected 1 synclet containerUpdate, actual: %d", f.sCli.UpdateContainerCount)
	}
	if f.sCli.CommandsRunCount != 2 {
		t.Errorf("Expected 2 commands run by the synclet, actual: %d", f.sCli.CommandsRunCount)
	}

	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
	assert.False(t, f.sCli.UpdateContainerHotReload)
}

func TestContainerBuildSyncletHotReload(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	manifest := NewSanchoFastBuildManifest(f)
	iTarget := manifest.ImageTargetAt(0)
	fbInfo := iTarget.FastBuildInfo()
	fbInfo.HotReload = true
	manifest = manifest.WithImageTarget(iTarget.WithBuildDetails(fbInfo))
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, f.sCli.UpdateContainerHotReload)
}

func TestDockerBuildWithNestedFastBuildDeploysSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifestWithNestedFastBuild(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 docker push, actual: %d", f.docker.PushCount)
	}

	expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	if !strings.Contains(f.k8s.Yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
	}

	if !strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should deploy the synclet for a docker build with a nested fast build: %s", f.k8s.Yaml)
	}
}

func TestDockerBuildWithNestedFastBuildContainerUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	manifest := NewSanchoDockerBuildManifestWithNestedFastBuild(f)
	targets := buildTargets(manifest)
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
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
}

func TestIncrementalBuildFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	f.docker.ExecErrorToThrow = docker.ExitError{ExitCode: 1}

	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	msg := "Command failed with exit code: 1"
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

func TestIncrementalBuildKilled(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")

	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	f.docker.ExecErrorToThrow = docker.ExitError{ExitCode: build.TaskKillExitCode}

	manifest := NewSanchoFastBuildManifest(f)
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	f.docker.ExecErrorToThrow = errors.New("some random error")

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())

	manifest := NewSanchoFastBuildManifest(f)
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	f.docker.ExecErrorToThrow = DontFallBackErrorf("i'm melllting")

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())

	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err == nil {
		t.Errorf("Expected this error to fail fallback tester and propogate back up")
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
}

func TestIncrementalBuildTwice(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
	aPath := f.WriteFile("a.txt", "a")
	bPath := f.WriteFile("b.txt", "b")

	firstState := resultToStateSet(alreadyBuiltSet, []string{aPath}, f.deployInfo())
	firstResult, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, firstState)
	if err != nil {
		t.Fatal(err)
	}

	id := manifest.ImageTargetAt(0).ID()
	rSet := firstResult[id].FilesReplacedSet
	if len(rSet) != 1 || !rSet[aPath] {
		t.Errorf("Expected replaced set with a.txt, actual: %v", rSet)
	}

	secondState := resultToStateSet(firstResult, []string{bPath}, f.deployInfo())
	secondResult, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, secondState)
	if err != nil {
		t.Fatal(err)
	}

	rSet = secondResult[id].FilesReplacedSet
	if len(rSet) != 2 || !rSet[aPath] || !rSet[bPath] {
		t.Errorf("Expected replaced set with a.txt, b.txt, actual: %v", rSet)
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
func TestIncrementalBuildTwiceDeadPod(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	manifest := NewSanchoFastBuildManifest(f)
	targets := buildTargets(manifest)
	aPath := f.WriteFile("a.txt", "a")
	bPath := f.WriteFile("b.txt", "b")

	firstState := resultToStateSet(alreadyBuiltSet, []string{aPath}, f.deployInfo())
	firstResult, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, firstState)
	if err != nil {
		t.Fatal(err)
	}

	id := manifest.ImageTargetAt(0).ID()
	rSet := firstResult[id].FilesReplacedSet
	if len(rSet) != 1 || !rSet[aPath] {
		t.Errorf("Expected replaced set with a.txt, actual: %v", rSet)
	}

	// Kill the pod
	f.docker.ExecErrorToThrow = fmt.Errorf("Dead pod")

	secondState := resultToStateSet(firstResult, []string{bPath}, f.deployInfo())
	secondResult, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, secondState)
	if err != nil {
		t.Fatal(err)
	}

	rSet = secondResult[id].FilesReplacedSet
	if len(rSet) != 0 {
		t.Errorf("Expected empty replaced set, actual: %v", rSet)
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

	// Make sure the right files were pushed to docker.
	tr := tar.NewReader(f.docker.BuildOptions.Context)
	testutils.AssertFilesInTar(t, tr, []expectedFile{
		expectedFile{
			Path: "Dockerfile",
			Contents: `FROM gcr.io/some-project-162817/sancho:deadbeef
LABEL "tilt.buildMode"="existing"
ADD . /
RUN ["go", "install", "github.com/windmilleng/sancho"]`,
		},
		expectedFile{
			Path:     "/go/src/github.com/windmilleng/sancho/a.txt",
			Contents: "a",
		},
		expectedFile{
			Path:     "/go/src/github.com/windmilleng/sancho/b.txt",
			Contents: "b",
		},
	})
}

func TestIgnoredFiles(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	manifest := NewSanchoFastBuildManifest(f)

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
			Path:     "go/src/github.com/windmilleng/sancho/a.txt",
			Contents: "a",
		},
		expectedFile{
			Path:    "go/src/github.com/windmilleng/sancho/.git/index",
			Missing: true,
		},
		expectedFile{
			Path:    "go/src/github.com/windmilleng/sancho/Tiltfile",
			Missing: true,
		},
	})
}

func TestCustomBuildWithFastBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithFastBuild(f)
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

	if !strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should deploy the synclet for a custom build with a fast build: %s", f.k8s.Yaml)
	}
}

func TestCustomBuildWithoutFastBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
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

	if strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should not deploy the synclet for a custom build: %s", f.k8s.Yaml)
	}
}

func TestCustomBuildDeterministicTag(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
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

	if strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should not deploy the synclet for a custom build: %s", f.k8s.Yaml)
	}
}

func TestContainerBuildMultiStage(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	manifest := NewSanchoFastMultiStageManifest(f)
	targets := buildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())

	// There are two image targets, and only the second one is dirty
	iTargetID := targets[0].ID()
	firstResult := store.NewImageBuildResult(iTargetID, container.MustParseNamedTagged("sancho-base:tilt-prebuilt"))
	bs[iTargetID] = store.NewBuildState(firstResult, nil)

	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))
	f.assertContainerRestarts(1)

	// The BuildComplete action handler expects to get exactly one result
	_, hasResult0 := result[manifest.ImageTargetAt(0).ID()]
	assert.False(t, hasResult0)
	_, hasResult1 := result[manifest.ImageTargetAt(1).ID()]
	assert.True(t, hasResult1)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
}

func TestDockerComposeImageBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoFastBuildDCManifest(f)
	targets := buildTargets(manifest)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	if strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should not deploy the synclet for a docker-compose build: %s", f.k8s.Yaml)
	}
	assert.Len(t, f.dcCli.UpCalls, 1)
}

func TestDockerComposeFastBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	manifest := NewSanchoFastBuildDCManifest(f)
	targets := buildTargets(manifest)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	assert.Equal(t, 1, f.docker.CopyCount)
	assert.Equal(t, 1, len(f.docker.ExecCalls))
	assert.Equal(t, 0, f.sCli.UpdateContainerCount)
	if strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName) {
		t.Errorf("Should not deploy the synclet for a docker-compose build: %s", f.k8s.Yaml)
	}
	f.assertContainerRestarts(1)
}

func TestReturnLastUnexpectedError(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoFastBuildDCManifest(f)
	targets := buildTargets(manifest)

	_, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}

}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake Client and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	docker *docker.FakeClient
	k8s    *k8s.FakeK8sClient
	sCli   *synclet.FakeSyncletClient
	bd     BuildAndDeployer
	st     *store.Store
	dcCli  *dockercompose.FakeDCClient
	logs   *bytes.Buffer
}

func newBDFixture(t *testing.T, env k8s.Env) *bdFixture {
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
	logs := new(bytes.Buffer)
	ctx := output.ForkedCtxForTest(logs)
	k8s := k8s.NewFakeK8sClient()
	sCli := synclet.NewFakeSyncletClient()
	mode := UpdateModeFlag(UpdateModeAuto)
	dcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	kp := &fakeKINDPusher{}
	bd, err := provideBuildAndDeployer(ctx, docker, k8s, dir, env, mode, sCli, dcc, fakeClock{now: time.Unix(1551202573, 0)}, kp)
	if err != nil {
		t.Fatal(err)
	}

	st, _ := store.NewStoreForTesting()

	return &bdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		docker:         docker,
		k8s:            k8s,
		sCli:           sCli,
		bd:             bd,
		st:             st,
		dcCli:          dcc,
		logs:           logs,
	}
}

func (f *bdFixture) deployInfo() store.DeployInfo {
	return store.DeployInfo{
		PodID:         "pod-id",
		ContainerID:   k8s.MagicTestContainerID,
		ContainerName: "container-name",
	}
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

func resultToStateSet(resultSet store.BuildResultSet, files []string, deploy store.DeployInfo) store.BuildStateSet {
	stateSet := store.BuildStateSet{}
	for id, result := range resultSet {
		state := store.NewBuildState(result, files).WithDeployTarget(deploy)
		stateSet[id] = state
	}
	return stateSet
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }
