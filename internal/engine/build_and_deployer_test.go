package engine

import (
	context "context"
	"fmt"
	"strings"
	"testing"

	build "github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils"
	dirs "github.com/windmilleng/wmclient/pkg/dirs"
)

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, nil, nil)
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

	_, err := f.bd.BuildAndDeploy(f.Ctx(), SanchoService, nil, nil)
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

func TestMinikubePortForwardToLB(t *testing.T) {
	f := newBDFixture(t, k8s.EnvMinikube)
	defer f.TearDown()

	_, err := f.bd.BuildAndDeploy(f.Ctx(), BlorgBackendService, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(f.k8s.lbs) != 1 {
		t.Fatalf("Expected 1 loadbalancer, actual %d: %+v", len(f.k8s.lbs), f.k8s.lbs)
	}

	lb := f.k8s.lbs[0]
	if lb.Name != "devel-nick-lb-blorg-be" || lb.Ports[0] != 8080 {
		t.Errorf("Unexpected loadbalancer: %+v", lb)
	}
}

// The API boundaries between BuildAndDeployer and the Builder aren't obvious and
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
	lbs  []k8s.LoadBalancer
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

func (c *FakeK8sClient) PortForward(ctx context.Context, lb k8s.LoadBalancer) error {
	c.lbs = append(c.lbs, lb)
	return nil
}

func (c *FakeK8sClient) BlockOnBackgroundProcesses() {
}
