package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

var cID = k8s.ContainerID("test_container")
var alreadyBuilt = BuildResult{Container: cID}

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, BuildStateClean)
	if err != nil {
		t.Fatal(err)
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

	_, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, BuildStateClean)
	if err != nil {
		t.Fatal(err)
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

	_, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, NewBuildState(alreadyBuilt))
	if err != nil {
		t.Fatal(err)
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

	nt, err := k8s.ParseNamedTagged("gcr.io/some-project-162817/sancho:foo")
	if err != nil {
		t.Fatal(err)
	}
	br := BuildResult{
		Image: nt,
	}

	newBR, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, NewBuildState(br))
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}

	expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	if !strings.Contains(f.k8s.yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.yaml)
	}

	if newBR.Container != k8s.ContainerID("") {
		t.Errorf("Expected container to be empty, got %s", newBR.Container)
	}
}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake DockerClient and K8sClient
type bdFixture struct {
	*testutils.TempDirFixture
	docker *build.FakeDockerClient
	k8s    *FakeK8sClient
	bd     BuildAndDeployer
}

func newBDFixture(t *testing.T, env k8s.Env) *bdFixture {
	t.Helper()
	f := testutils.NewTempDirFixture(t)
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
	bd, err := provideBuildAndDeployer(f.Ctx(), docker, k8s, dir, env, false)
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

func (c *FakeK8sClient) applyWasCalled() bool {
	return c.yaml != ""
}
