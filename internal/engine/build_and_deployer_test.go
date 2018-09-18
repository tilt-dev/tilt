package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

var imageID, _ = reference.ParseNamed("gcr.io/some-project-162817/sancho:deadbeef")
var alreadyBuilt = BuildResult{Image: imageID.(reference.NamedTagged)}

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

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, BuildStateClean)
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
	if !strings.Contains(f.k8s.yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.yaml)
	}
}

func TestDockerForMacDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, BuildStateClean)
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
	if !strings.Contains(f.k8s.yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.yaml)
	}
}

func TestIncrementalBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForManifest(SanchoManifest, alreadyBuilt)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, NewBuildState(alreadyBuilt))
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
	if len(f.docker.RestartsByContainer) != 1 {
		t.Errorf("Expected 1 container to be restarted, actual: %d", len(f.docker.RestartsByContainer))
	}
}

func TestIncrementalBuildFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForManifest(SanchoManifest, alreadyBuilt)
	defer f.TearDown()

	ctx := output.CtxForTest()

	f.docker.ExecErrorToThrow = build.ExitError{ExitCode: 1}
	_, err := f.bd.BuildAndDeploy(ctx, SanchoManifest, NewBuildState(alreadyBuilt))
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
	if len(f.docker.RestartsByContainer) != 0 {
		t.Errorf("Expected 0 containers to be restarted, actual: %d", len(f.docker.RestartsByContainer))
	}
}

func TestFallBackToImageDeploy(t *testing.T) {
	f := newBDFallbackFixture(t, k8s.EnvDockerDesktop).withContainerForManifest(SanchoManifest, alreadyBuilt)
	defer f.TearDown()

	f.docker.ExecErrorToThrow = errors.New("some random error")

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, NewBuildState(alreadyBuilt))
	if err != nil {
		t.Fatal(err)
	}

	if len(f.docker.RestartsByContainer) != 0 {
		t.Errorf("Expected no docker container restarts, actual: %d", len(f.docker.RestartsByContainer))
	}
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

}

func TestNoFallbackForCertainErrors(t *testing.T) {
	f := newBDFallbackFixture(t, k8s.EnvDockerDesktop).withContainerForManifest(SanchoManifest, alreadyBuilt)
	defer f.TearDown()
	f.docker.ExecErrorToThrow = errors.New(dontFallBackErrStr)

	_, err := f.bd.BuildAndDeploy(f.ctx, SanchoManifest, NewBuildState(alreadyBuilt))
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
	t.Skip()
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForManifest(SanchoManifest, alreadyBuilt)
	defer f.TearDown()
	ctx := output.CtxForTest()

	manifest := SanchoManifest
	manifest.Mounts[0].LocalPath = f.Path()
	aPath := filepath.Join(f.Path(), "a.txt")
	bPath := filepath.Join(f.Path(), "b.txt")
	ioutil.WriteFile(aPath, []byte("a"), os.FileMode(0777))
	ioutil.WriteFile(bPath, []byte("b"), os.FileMode(0777))

	firstState := NewBuildState(alreadyBuilt)
	firstState.filesChangedSet[aPath] = true

	firstResult, err := f.bd.BuildAndDeploy(ctx, SanchoManifest, firstState)
	if err != nil {
		t.Fatal(err)
	}

	rSet := firstResult.FilesReplacedSet
	if len(rSet) != 1 || !rSet[aPath] {
		t.Errorf("Expected replaced set with a.txt, actual: %v", rSet)
	}

	secondState := NewBuildState(firstResult)
	secondState.filesChangedSet[bPath] = true
	secondResult, err := f.bd.BuildAndDeploy(ctx, SanchoManifest, secondState)
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
	if len(f.docker.RestartsByContainer) != 2 {
		t.Errorf("Expected 2 container to be restarted, actual: %d", len(f.docker.RestartsByContainer))
	}
}

// Kill the pod after the first container update,
// and make sure the next image build gets the right file updates.
func TestIncrementalBuildTwiceDeadPod(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop).withContainerForManifest(SanchoManifest, alreadyBuilt)
	defer f.TearDown()
	ctx := output.CtxForTest()

	manifest := SanchoManifest
	manifest.Mounts[0].LocalPath = f.Path()
	aPath := filepath.Join(f.Path(), "a.txt")
	bPath := filepath.Join(f.Path(), "b.txt")
	ioutil.WriteFile(aPath, []byte("a"), os.FileMode(0777))
	ioutil.WriteFile(bPath, []byte("b"), os.FileMode(0777))

	firstState := NewBuildState(alreadyBuilt)
	firstState.filesChangedSet[aPath] = true

	firstResult, err := f.bd.BuildAndDeploy(ctx, SanchoManifest, firstState)
	if err != nil {
		t.Fatal(err)
	}

	rSet := firstResult.FilesReplacedSet
	if len(rSet) != 1 || !rSet[aPath] {
		t.Errorf("Expected replaced set with a.txt, actual: %v", rSet)
	}

	// Kill the pod
	f.docker.ExecErrorToThrow = fmt.Errorf("Dead pod")

	secondState := NewBuildState(firstResult)
	secondState.filesChangedSet[bPath] = true
	secondResult, err := f.bd.BuildAndDeploy(ctx, SanchoManifest, secondState)
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
	if len(f.docker.RestartsByContainer) != 1 {
		t.Errorf("Expected 1 container to be restarted, actual: %d", len(f.docker.RestartsByContainer))
	}

	// Make sure the right files were pushed to docker.
	tarBB := bytes.NewBuffer(nil)
	io.Copy(tarBB, f.docker.BuildOptions.Context)
	tarBytes := tarBB.Bytes()

	// TODO(nick): This is super-janky. Add a function that can assert multiple files in a tarball
	testutils.AssertFileInTar(t, tar.NewReader(bytes.NewBuffer(tarBytes)), testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `FROM gcr.io/some-project-162817/sancho:deadbeef
LABEL "tilt.buildMode"="existing"
ADD . /
RUN ["go", "install", "github.com/windmilleng/sancho"]`,
	})
	testutils.AssertFileInTar(t, tar.NewReader(bytes.NewBuffer(tarBytes)), testutils.ExpectedFile{
		Path:     "go/src/github.com/windmilleng/sancho/a.txt",
		Contents: "a",
	})
	testutils.AssertFileInTar(t, tar.NewReader(bytes.NewBuffer(tarBytes)), testutils.ExpectedFile{
		Path:     "go/src/github.com/windmilleng/sancho/b.txt",
		Contents: "b",
	})
}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake DockerClient and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	docker *build.FakeDockerClient
	k8s    *FakeK8sClient
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
	docker := build.NewFakeDockerClient()
	docker.ContainerListOutput = map[string][]types.Container{
		"pod": []types.Container{
			types.Container{
				ID: "testcontainer",
			},
		},
	}
	ctx := output.CtxForTest()
	k8s := &FakeK8sClient{}
	bd, err := provideBuildAndDeployer(output.CtxForTest(), docker, k8s, dir, env, synclet.NewFakeSyncletClient(), fallbackFn)
	if err != nil {
		t.Fatal(err)
	}

	return &bdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		docker:         docker,
		k8s:            k8s,
		bd:             bd,
	}
}

// Ensure that the BuildAndDeployer has container information attached for the given manifest.
func (f *bdFixture) withContainerForManifest(manifest model.Manifest, build BuildResult) *bdFixture {
	f.bd.PostProcessBuild(f.ctx, manifest, build)
	return f
}

type FakeK8sClient struct {
	yaml string
	lb   k8s.LoadBalancer
}

var _ k8s.Client = &FakeK8sClient{}

func (c *FakeK8sClient) OpenService(ctx context.Context, lb k8s.LoadBalancer) error {
	c.lb = lb
	return nil
}

func (c *FakeK8sClient) Apply(ctx context.Context, entities []k8s.K8sEntity) error {
	yaml, err := k8s.SerializeYAML(entities)
	if err != nil {
		return fmt.Errorf("kubectl apply: %v", err)
	}
	c.yaml = yaml
	return nil
}

func (c *FakeK8sClient) Delete(ctx context.Context, entities []k8s.K8sEntity) error {
	return nil
}

func (c *FakeK8sClient) PodWithImage(ctx context.Context, image reference.NamedTagged) (k8s.PodID, error) {
	return k8s.PodID("pod"), nil
}

func (c *FakeK8sClient) PollForPodWithImage(ctx context.Context, image reference.NamedTagged, timeout time.Duration) (k8s.PodID, error) {
	return c.PodWithImage(ctx, image)
}

func (c *FakeK8sClient) applyWasCalled() bool {
	return c.yaml != ""
}

func (c *FakeK8sClient) FindAppByNode(ctx context.Context, appName string, nodeID k8s.NodeID) (k8s.PodID, error) {
	return k8s.PodID("pod2"), nil
}

func (c *FakeK8sClient) GetNodeForPod(ctx context.Context, podID k8s.PodID) (k8s.NodeID, error) {
	return k8s.NodeID("node"), nil
}

func (c *FakeK8sClient) ForwardPort(ctx context.Context, namespace string, podID k8s.PodID, remotePort int) (int, func(), error) {
	return 0, nil, nil
}
