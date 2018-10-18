package engine

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

var imageID = k8s.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef")
var alreadyBuilt = store.BuildResult{Image: imageID}

type expectedFile = testutils.ExpectedFile

var dontFallBackErrStr = "don't fall back"

func TestShouldImageBuild(t *testing.T) {
	m := model.Mount{
		LocalPath:     "asdf",
		ContainerPath: "blah",
	}
	_, pathMapErr := build.FilesToPathMappings([]string{"a"}, []model.Mount{m})
	if assert.Error(t, pathMapErr) {
		assert.False(t, shouldImageBuild(pathMapErr))
	}

	s := model.Manifest{Name: "many errors"}
	validateErr := s.Validate()
	if assert.Error(t, validateErr) {
		assert.False(t, shouldImageBuild(validateErr))
	}

	err := fmt.Errorf("hello world")
	assert.True(t, shouldImageBuild(err))
}

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.BuildStateClean)
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

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.BuildStateClean)
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

	result, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.BuildStateClean)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "sancho-ns", string(result.Namespace))

	result, err = f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.NewBuildState(result, nil))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "sancho-ns", string(result.Namespace))
	assert.Equal(t, "sancho-ns", string(f.sCli.Namespace))
	assert.Equal(t, "sancho-ns", string(f.k8s.LastPodQueryNamespace))
}

func TestIncrementalBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.NewBuildState(alreadyBuilt, nil))
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
}

func TestIncrementalBuildWaitsForPostProcess(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	f.k8s.SetPollForPodsWithImageDelay(time.Second * 1)

	res, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.BuildStateClean)
	if err != nil {
		t.Fatal(err)
	}

	// Expected behavior: this build call waits on the PostProcess initiated at the end
	// of the previous build, and when that info is available, does a container build
	_, err = f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.NewBuildState(res, nil))
	if err != nil {
		t.Fatal(err)
	}
	if f.docker.BuildCount != 1 { // initial build
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 1 { // initial build
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}
	if f.sCli.UpdateContainerCount != 1 { // second build via synclet
		t.Errorf("Expected 1 UpdateContainer count via synclet, actual: %d", f.sCli.UpdateContainerCount)
	}
}

func TestIncrementalBuildFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()

	ctx := output.CtxForTest()

	f.docker.ExecErrorToThrow = docker.ExitError{ExitCode: 1}
	_, err := f.bd.BuildAndDeploy(ctx, SanchoManifest, store.NewBuildState(alreadyBuilt, nil))
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

func TestFallBackToImageDeploy(t *testing.T) {
	f := newBDFallbackFixture(t, k8s.EnvDockerDesktop).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()

	f.docker.ExecErrorToThrow = errors.New("some random error")

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.NewBuildState(alreadyBuilt, nil))
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(0)
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

}

func TestNoFallbackForCertainErrors(t *testing.T) {
	f := newBDFallbackFixture(t, k8s.EnvDockerDesktop).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()
	f.docker.ExecErrorToThrow = errors.New(dontFallBackErrStr)

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.NewBuildState(alreadyBuilt, nil))
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
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()
	ctx := output.CtxForTest()

	manifest := NewSanchoManifest()
	manifest.Mounts[0].LocalPath = f.Path()
	aPath := filepath.Join(f.Path(), "a.txt")
	bPath := filepath.Join(f.Path(), "b.txt")
	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	firstState := store.NewBuildState(alreadyBuilt, []string{aPath})

	firstResult, err := f.bd.BuildAndDeploy(ctx, manifest, firstState)
	if err != nil {
		t.Fatal(err)
	}

	rSet := firstResult.FilesReplacedSet
	if len(rSet) != 1 || !rSet[aPath] {
		t.Errorf("Expected replaced set with a.txt, actual: %v", rSet)
	}

	secondState := store.NewBuildState(firstResult, []string{bPath})
	secondResult, err := f.bd.BuildAndDeploy(ctx, manifest, secondState)
	if err != nil {
		t.Fatal(err)
	}

	rSet = secondResult.FilesReplacedSet
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
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()
	ctx := output.CtxForTest()

	manifest := NewSanchoManifest()
	manifest.Mounts[0].LocalPath = f.Path()
	aPath := filepath.Join(f.Path(), "a.txt")
	bPath := filepath.Join(f.Path(), "b.txt")
	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	firstState := store.NewBuildState(alreadyBuilt, []string{aPath})
	firstResult, err := f.bd.BuildAndDeploy(ctx, manifest, firstState)
	if err != nil {
		t.Fatal(err)
	}

	rSet := firstResult.FilesReplacedSet
	if len(rSet) != 1 || !rSet[aPath] {
		t.Errorf("Expected replaced set with a.txt, actual: %v", rSet)
	}

	// Kill the pod
	f.docker.ExecErrorToThrow = fmt.Errorf("Dead pod")

	secondState := store.NewBuildState(firstResult, []string{bPath})
	secondResult, err := f.bd.BuildAndDeploy(ctx, manifest, secondState)
	if err != nil {
		t.Fatal(err)
	}

	rSet = secondResult.FilesReplacedSet
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
			Path:     "go/src/github.com/windmilleng/sancho/a.txt",
			Contents: "a",
		},
		expectedFile{
			Path:     "go/src/github.com/windmilleng/sancho/b.txt",
			Contents: "b",
		},
	})
}

func TestBaDForgetsImages(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE).withContainerForBuild(alreadyBuilt)
	defer f.TearDown()

	// make sBaD return an error so that we fall back to iBaD and get a new image id
	f.sCli.UpdateContainerErrorToReturn = errors.New("blah")

	f.k8s.SetPodsWithImageResp(pod1)

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, store.NewBuildState(alreadyBuilt, nil))
	if err != nil {
		t.Fatal(err)
	}

	if f.sCli.ClosedCount != 1 {
		t.Errorf("Expected 1 synclet client close, actual: %d", f.sCli.ClosedCount)
	}
}

func TestIgnoredFiles(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	ctx := output.CtxForTest()

	manifest := NewSanchoManifest()
	manifest.Mounts[0].LocalPath = f.Path()

	manifest.Repos = []model.LocalGithubRepo{
		model.LocalGithubRepo{
			LocalPath:            f.Path(),
			DockerignoreContents: "",
			GitignoreContents:    "",
		},
	}
	manifest.TiltFilename = filepath.Join(f.Path(), "Tiltfile")

	f.WriteFile("Tiltfile", "# hello world")
	f.WriteFile("a.txt", "a")
	f.WriteFile(".git/index", "garbage")

	_, err := f.bd.BuildAndDeploy(ctx, manifest, store.BuildStateClean)
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

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake DockerClient and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	docker *docker.FakeDockerClient
	k8s    *k8s.FakeK8sClient
	sCli   *synclet.FakeSyncletClient
	bd     BuildAndDeployer
}

func shouldFallBack(err error) bool {
	if strings.Contains(err.Error(), dontFallBackErrStr) {
		return false
	}
	return true
}

func newBDFallbackFixture(t *testing.T, env k8s.Env) *bdFixture {
	return newBDFixtureHelper(t, env, shouldFallBack)
}

func newBDFixture(t *testing.T, env k8s.Env) *bdFixture {
	return newBDFixtureHelper(t, env, shouldImageBuild)
}

func newBDFixtureHelper(t *testing.T, env k8s.Env, fallbackFn FallbackTester) *bdFixture {
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(f.Path())
	docker := docker.NewFakeDockerClient()
	docker.ContainerListOutput = map[string][]types.Container{
		"pod": []types.Container{
			types.Container{
				ID: k8s.MagicTestContainerID,
			},
		},
	}
	ctx := output.CtxForTest()
	k8s := k8s.NewFakeK8sClient()
	sCli := synclet.NewFakeSyncletClient()
	mode := UpdateModeFlag(UpdateModeAuto)
	bd, err := provideBuildAndDeployer(output.CtxForTest(), docker, k8s, dir, env, mode, sCli, fallbackFn)
	if err != nil {
		t.Fatal(err)
	}

	return &bdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		docker:         docker,
		k8s:            k8s,
		sCli:           sCli,
		bd:             bd,
	}
}

// Ensure that the BuildAndDeployer has container information attached for the given manifest.
func (f *bdFixture) withContainerForBuild(build store.BuildResult) *bdFixture {
	f.bd.PostProcessBuild(f.ctx, build, store.BuildResult{})
	return f
}

func (f *bdFixture) assertContainerRestarts(count int) {
	// Ensure that MagicTestContainerID was the only container id that saw
	// restarts, and that it saw the right number of restarts.
	expected := map[string]int{}
	if count != 0 {
		expected[string(k8s.MagicTestContainerID)] = count
	}
	assert.Equal(f.T(), expected, f.docker.RestartsByContainer)
}
