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

	PodsWithImageResp         PodID
	PollForPodsWithImageDelay time.Duration

	LastPodQueryNamespace Namespace
	LastPodQueryImage     reference.NamedTagged
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

func (c *FakeK8sClient) SetPodsWithImageResp(pID PodID) {
	c.PodsWithImageResp = pID
}

func (c *FakeK8sClient) WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

func (c *FakeK8sClient) PodByID(ctx context.Context, pID PodID, n Namespace) (*v1.Pod, error) {
	return nil, nil
}

func (c *FakeK8sClient) PodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []LabelPair) ([]v1.Pod, error) {
	c.LastPodQueryImage = image
	c.LastPodQueryNamespace = n

	status := v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				ContainerID: "docker://tilt-testcontainer",
				Image:       image.String(),
				Ready:       true,
			},
			{
				ContainerID: "docker://tilt-testservlet",
				// can't use the constants in synclet because that would create a dep cycle
				Image: "gcr.io/windmill-public-containers/tilt-synclet:latest",
				Ready: true,
			},
		},
	}
	if !c.PodsWithImageResp.Empty() {
		res := c.PodsWithImageResp
		c.PodsWithImageResp = ""
		return []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   string(res),
					Labels: makeLabelSet(labels),
				},
				Status: status,
			},
		}, nil
	}
	return []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pod",
				Labels: makeLabelSet(labels),
			},
			Status: status,
		},
	}, nil
}

func (c *FakeK8sClient) SetPollForPodsWithImageDelay(dur time.Duration) {
	c.PollForPodsWithImageDelay = dur
}

func (c *FakeK8sClient) PollForPodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []LabelPair, timeout time.Duration) ([]v1.Pod, error) {
	defer c.SetPollForPodsWithImageDelay(0)

	if c.PollForPodsWithImageDelay > timeout {
		return nil, fmt.Errorf("timeout polling for pod (delay %s > timeout %s)",
			c.PollForPodsWithImageDelay.String(), timeout.String())
	}

	time.Sleep(c.PollForPodsWithImageDelay)
	return c.PodsWithImage(ctx, image, n, labels)
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

func (c *FakeK8sClient) ForwardPort(ctx context.Context, namespace Namespace, podID PodID, remotePort int) (int, func(), error) {
	return 0, nil, nil
}
