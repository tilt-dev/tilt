package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/distribution/reference"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// A magic constant. If the docker client returns this constant, we always match
// even if the container doesn't have the correct image name.
const MagicTestContainerID = "tilt-testcontainer"

var _ Client = &FakeK8sClient{}

type FakeK8sClient struct {
	Yaml string
	Lb   LoadBalancerSpec

	PodsWithImageResp         PodID
	PodsWithImageError        error
	PollForPodsWithImageDelay time.Duration

	LastPodQueryNamespace Namespace
	LastPodQueryImage     reference.NamedTagged

	PodLogs string
}

func (c *FakeK8sClient) WatchServices(ctx context.Context, lps []LabelPair) (<-chan *v1.Service, error) {
	return nil, nil
}

func (c *FakeK8sClient) WatchPods(ctx context.Context, lps []LabelPair) (<-chan *v1.Pod, error) {
	return nil, nil
}

func NewFakeK8sClient() *FakeK8sClient {
	return &FakeK8sClient{}
}

func (c *FakeK8sClient) ResolveLoadBalancer(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error) {
	c.Lb = lb
	return LoadBalancer{}, nil
}

func (c *FakeK8sClient) Upsert(ctx context.Context, entities []K8sEntity) error {
	yaml, err := SerializeYAML(entities)
	if err != nil {
		return fmt.Errorf("kubectl apply: %v", err)
	}
	c.Yaml = yaml
	return nil
}

func (c *FakeK8sClient) SetPodsWithImageResp(pID PodID) {
	c.PodsWithImageResp = pID
}

func (c *FakeK8sClient) WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

func (c *FakeK8sClient) ContainerLogs(ctx context.Context, pID PodID, cName ContainerName, n Namespace) (io.ReadCloser, error) {
	return BufferCloser{bytes.NewBufferString(c.PodLogs)}, nil
}

func (c *FakeK8sClient) PodByID(ctx context.Context, pID PodID, n Namespace) (*v1.Pod, error) {
	return nil, nil
}

func (c *FakeK8sClient) PodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []LabelPair) ([]v1.Pod, error) {
	c.LastPodQueryImage = image
	c.LastPodQueryNamespace = n

	if c.PodsWithImageError != nil {
		return nil, c.PodsWithImageError
	}

	status := v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				ContainerID: "docker://" + MagicTestContainerID,
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

type BufferCloser struct {
	*bytes.Buffer
}

func (b BufferCloser) Close() error {
	return nil
}

var _ io.ReadCloser = BufferCloser{}
