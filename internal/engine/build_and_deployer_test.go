package engine

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

var cID = k8s.ContainerID("test_container")
var alreadyBuilt = BuildResult{Container: cID}

var dontFallBackErrStr = "don't fall back"

func TestShouldImageBuild(t *testing.T) {
	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: "asdf"},
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

	_, err := f.bd.BuildAndDeploy(output.CtxForTest(), SanchoManifest, BuildStateClean)
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

	_, err := f.bd.BuildAndDeploy(output.CtxForTest(), SanchoManifest, BuildStateClean)
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(output.CtxForTest(), SanchoManifest, NewBuildState(alreadyBuilt))
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
		t.Errorf("Expected 1 copy to docker container call, actual: %d", f.docker.PushCount)
	}
	if len(f.docker.ExecCalls) != 1 {
		t.Errorf("Expected 1 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	if len(f.docker.RestartsByContainer) != 1 {
		t.Errorf("Expected 1 container to be restarted, actual: %d", len(f.docker.RestartsByContainer))
	}
}

func TestFallBackToImageDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	f.docker.ExecErrorToThrow = errors.New("some random error")

	_, err := f.bd.BuildAndDeploy(output.CtxForTest(), SanchoManifest, NewBuildState(alreadyBuilt))
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	f.docker.ExecErrorToThrow = errors.New(dontFallBackErrStr)

	// Malformed manifest (it's missing fields) will trip a validate error; we
	// should NOT fall back to image build, but rather, return the error.
	_, err := f.bd.BuildAndDeploy(output.CtxForTest(), SanchoManifest, NewBuildState(alreadyBuilt))
	if err == nil {
		t.Errorf("Expected bad manifest error to propogate back up")
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake DockerClient and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
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

func newBDFixture(t *testing.T, env k8s.Env) *bdFixture {
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
	k8s := &FakeK8sClient{}
	bd, err := provideBuildAndDeployer(output.CtxForTest(), docker, k8s, dir, env, synclet.NewFakeSyncletClient(), shouldFallBack)
	if err != nil {
		t.Fatal(err)
	}

	return &bdFixture{
		TempDirFixture: f,
		docker:         docker,
		k8s:            k8s,
		bd:             bd,
	}
}

type FakeK8sClient struct {
	yaml               string
	lb                 k8s.LoadBalancer
	podWithImageExists bool
}

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
	if !c.podWithImageExists {
		return k8s.PodID(""), fmt.Errorf("Pod not found")
	}

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
