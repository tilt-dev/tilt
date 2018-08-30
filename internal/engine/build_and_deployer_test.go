package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/docker/distribution/reference"
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

// TODO(maia): make this test go. (Expect it to call ContainerBuildAndDeployer stuff.)
func TestIncrementalBuild(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, NewBuildState(alreadyBuilt))
	if err != nil {
		t.Fatal(err)
	}

	// if f.docker.PushCount != 0 {
	// 	t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	// }
	//
	// expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	// if !strings.Contains(f.k8s.yaml, expectedYaml) {
	// 	t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.yaml)
	// }
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
	f := testutils.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(f.Path())
	docker := build.NewFakeDockerClient()
	k8s := &FakeK8sClient{}
	bd, err := provideBuildAndDeployer(f.Ctx(), docker, k8s, dir, env)
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
	yaml string
	lb   k8s.LoadBalancer
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

func (c *FakeK8sClient) PodWithImage(ctx context.Context, image reference.Named) (k8s.PodID, error) {
	return "", fmt.Errorf("TODO(maia): not implemented")
}

func (c *FakeK8sClient) applyWasCalled() bool {
	return c.yaml != ""
}
