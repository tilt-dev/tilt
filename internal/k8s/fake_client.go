package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
)

var _ Client = &FakeK8sClient{}

type FakeK8sClient struct {
	Yaml string
	Lb   LoadBalancerSpec
}

func NewFakeK8sClient() *FakeK8sClient {
	return &FakeK8sClient{}
}

func (c *FakeK8sClient) ResolveLoadBalancer(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error) {
	c.Lb = lb
	return LoadBalancer{}, nil
}

func (c *FakeK8sClient) Apply(ctx context.Context, entities []K8sEntity) error {
	yaml, err := SerializeYAML(entities)
	if err != nil {
		return fmt.Errorf("kubectl apply: %v", err)
	}
	c.Yaml = yaml
	return nil
}

func (c *FakeK8sClient) Delete(ctx context.Context, entities []K8sEntity) error {
	return nil
}

func (c *FakeK8sClient) PodWithImage(ctx context.Context, image reference.NamedTagged) (PodID, error) {
	return PodID("pod"), nil
}

func (c *FakeK8sClient) PollForPodWithImage(ctx context.Context, image reference.NamedTagged, timeout time.Duration) (PodID, error) {
	return c.PodWithImage(ctx, image)
}

func (c *FakeK8sClient) applyWasCalled() bool {
	return c.Yaml != ""
}

func (c *FakeK8sClient) FindAppByNode(ctx context.Context, nodeID NodeID, appName string, options FindAppByNodeOptions) (PodID, error) {
	return PodID("pod2"), nil
}

func (c *FakeK8sClient) GetNodeForPod(ctx context.Context, podID PodID) (NodeID, error) {
	return NodeID("node"), nil
}

func (c *FakeK8sClient) ForwardPort(ctx context.Context, namespace string, podID PodID, remotePort int) (int, func(), error) {
	return 0, nil, nil
}
