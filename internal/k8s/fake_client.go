package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

var _ Client = &FakeK8sClient{}

type FakeK8sClient struct {
	Yaml string
	Lb   LoadBalancerSpec

	PodWithImageResp         PodID
	PollForPodWithImageDelay time.Duration
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

func (c *FakeK8sClient) SetPodWithImageResp(pID PodID) {
	c.PodWithImageResp = pID
}

func (c *FakeK8sClient) WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

func (c *FakeK8sClient) PodByID(ctx context.Context, pID PodID, n Namespace) (*v1.Pod, error) {
	return nil, nil
}

func (c *FakeK8sClient) PodWithImage(ctx context.Context, image reference.NamedTagged, n Namespace) (*v1.Pod, error) {
	if !c.PodWithImageResp.Empty() {
		res := c.PodWithImageResp
		c.PodWithImageResp = ""
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: string(res)},
		}, nil
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod"},
	}, nil
}

func (c *FakeK8sClient) SetPollForPodWithImageDelay(dur time.Duration) {
	c.PollForPodWithImageDelay = dur
}

func (c *FakeK8sClient) PollForPodWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, timeout time.Duration) (*v1.Pod, error) {
	defer c.SetPollForPodWithImageDelay(0)

	if c.PollForPodWithImageDelay > timeout {
		return nil, fmt.Errorf("timeout polling for pod (delay %s > timeout %s)",
			c.PollForPodWithImageDelay.String(), timeout.String())
	}

	time.Sleep(c.PollForPodWithImageDelay)
	return c.PodWithImage(ctx, image, n)
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
