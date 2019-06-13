package k8s

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/windmilleng/tilt/internal/container"
)

// A magic constant. If the docker client returns this constant, we always match
// even if the container doesn't have the correct image name.
const MagicTestContainerID = "tilt-testcontainer"

var _ Client = &FakeK8sClient{}

// For keying PodLogsByPodAndContainer
type PodAndCName struct {
	PID   PodID
	CName container.Name
}

type FakeK8sClient struct {
	Yaml        string
	DeletedYaml string
	Lb          LoadBalancerSpec

	LastPodQueryNamespace Namespace
	LastPodQueryImage     reference.NamedTagged

	PodLogsByPodAndContainer map[PodAndCName]BufferCloser
	ContainerLogsError       error

	LastForwardPortPodID      PodID
	LastForwardPortRemotePort int

	podWatcherMu sync.Mutex
	podWatches   []fakePodWatch

	serviceWatcherMu sync.Mutex
	serviceWatches   []fakeServiceWatch

	everythingWatcherMu sync.Mutex
	everythingWatches   []fakeEverythingWatch
	EverythingWatchErr  error

	eventsCh       chan *v1.Event
	EventsWatchErr error

	UpsertError error
	Runtime     container.Runtime
	Registry    container.Registry
}

type fakeServiceWatch struct {
	ls labels.Selector
	ch chan *v1.Service
}

type fakePodWatch struct {
	ls labels.Selector
	ch chan *v1.Pod
}

type fakeEverythingWatch struct {
	ls labels.Selector
	ch chan watch.Event
}

func (c *FakeK8sClient) EmitService(ls labels.Selector, s *v1.Service) {
	c.podWatcherMu.Lock()
	defer c.podWatcherMu.Unlock()
	for _, w := range c.serviceWatches {
		if SelectorEqual(ls, w.ls) {
			w.ch <- s
		}
	}
}

func (c *FakeK8sClient) WatchServices(ctx context.Context, lps []model.LabelPair) (<-chan *v1.Service, error) {
	c.serviceWatcherMu.Lock()
	ch := make(chan *v1.Service, 20)
	ls := LabelPairsToSelector(lps)
	c.serviceWatches = append(c.serviceWatches, fakeServiceWatch{ls, ch})
	c.serviceWatcherMu.Unlock()

	go func() {
		// when ctx is canceled, remove the label selector from the list of watched label selectors
		<-ctx.Done()
		c.serviceWatcherMu.Lock()
		var newWatches []fakeServiceWatch
		for _, e := range c.serviceWatches {
			if !SelectorEqual(e.ls, ls) {
				newWatches = append(newWatches, e)
			}
		}
		c.serviceWatches = newWatches
		c.serviceWatcherMu.Unlock()
	}()
	return ch, nil
}

func (c *FakeK8sClient) WatchEvents(ctx context.Context) (<-chan *v1.Event, error) {
	if c.EventsWatchErr != nil {
		err := c.EventsWatchErr
		c.EventsWatchErr = nil
		return nil, err
	}

	return c.eventsCh, nil
}

func (c *FakeK8sClient) EmitEvent(ctx context.Context, evt *v1.Event) {
	c.eventsCh <- evt
}

// emits an event to chans returned by WatchEverything
func (c *FakeK8sClient) EmitEverything(ls labels.Selector, e watch.Event) {
	c.everythingWatcherMu.Lock()
	defer c.everythingWatcherMu.Unlock()
	for _, w := range c.everythingWatches {
		if SelectorEqual(ls, w.ls) {
			w.ch <- e
		}
	}
}

func (c *FakeK8sClient) WatchEverything(ctx context.Context, lps []model.LabelPair) (<-chan watch.Event, error) {
	if c.EverythingWatchErr != nil {
		err := c.EverythingWatchErr
		c.EverythingWatchErr = nil
		return nil, err
	}

	c.everythingWatcherMu.Lock()
	ch := make(chan watch.Event, 20)
	ls := LabelPairsToSelector(lps)
	c.everythingWatches = append(c.everythingWatches, fakeEverythingWatch{ls, ch})
	c.everythingWatcherMu.Unlock()

	go func() {
		// when ctx is canceled, remove the label selector from the list of watched label selectors
		<-ctx.Done()
		c.everythingWatcherMu.Lock()
		var newWatches []fakeEverythingWatch
		for _, e := range c.everythingWatches {
			if !SelectorEqual(e.ls, ls) {
				newWatches = append(newWatches, e)
			}
		}
		c.everythingWatches = newWatches
		c.everythingWatcherMu.Unlock()
	}()
	return ch, nil

}

func (c *FakeK8sClient) WatchedSelectors() []labels.Selector {
	c.podWatcherMu.Lock()
	defer c.podWatcherMu.Unlock()
	var ret []labels.Selector
	for _, w := range c.podWatches {
		ret = append(ret, w.ls)
	}
	return ret
}

func (c *FakeK8sClient) EmitPod(ls labels.Selector, p *v1.Pod) {
	c.podWatcherMu.Lock()
	defer c.podWatcherMu.Unlock()
	for _, w := range c.podWatches {
		if SelectorEqual(ls, w.ls) {
			w.ch <- p
		}
	}
}

func (c *FakeK8sClient) WatchPods(ctx context.Context, ls labels.Selector) (<-chan *v1.Pod, error) {
	c.podWatcherMu.Lock()
	ch := make(chan *v1.Pod, 20)
	c.podWatches = append(c.podWatches, fakePodWatch{ls, ch})
	c.podWatcherMu.Unlock()

	go func() {
		// when ctx is canceled, remove the label selector from the list of watched label selectors
		<-ctx.Done()
		c.podWatcherMu.Lock()
		var newWatches []fakePodWatch
		for _, e := range c.podWatches {
			if !SelectorEqual(e.ls, ls) {
				newWatches = append(newWatches, e)
			}
		}
		c.podWatches = newWatches
		c.podWatcherMu.Unlock()
	}()
	return ch, nil
}

func NewFakeK8sClient() *FakeK8sClient {
	return &FakeK8sClient{
		PodLogsByPodAndContainer: make(map[PodAndCName]BufferCloser),
		eventsCh:                 make(chan *v1.Event, 10),
	}
}

func (c *FakeK8sClient) TearDown() {
	if c.eventsCh != nil {
		close(c.eventsCh)
	}
}

func (c *FakeK8sClient) ConnectedToCluster(ctx context.Context) error {
	return nil
}

func (c *FakeK8sClient) Upsert(ctx context.Context, entities []K8sEntity) error {
	if c.UpsertError != nil {
		return c.UpsertError
	}
	yaml, err := SerializeSpecYAML(entities)
	if err != nil {
		return errors.Wrap(err, "kubectl apply")
	}
	c.Yaml = yaml
	return nil
}

func (c *FakeK8sClient) Delete(ctx context.Context, entities []K8sEntity) error {
	yaml, err := SerializeSpecYAML(entities)
	if err != nil {
		return errors.Wrap(err, "kubectl delete")
	}
	c.DeletedYaml = yaml
	return nil
}

func (c *FakeK8sClient) WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

func (c *FakeK8sClient) SetLogsForPodContainer(pID PodID, cName container.Name, logs string) {
	c.PodLogsByPodAndContainer[PodAndCName{pID, cName}] = BufferCloser{Buffer: bytes.NewBufferString(logs)}
}

func (c *FakeK8sClient) ContainerLogs(ctx context.Context, pID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error) {
	if c.ContainerLogsError != nil {
		return nil, c.ContainerLogsError
	}

	// If we have specific logs for this pod/container combo, return those
	if buf, ok := c.PodLogsByPodAndContainer[PodAndCName{pID, cName}]; ok {
		return buf, nil
	}

	return BufferCloser{Buffer: bytes.NewBuffer(nil)}, nil
}

func (c *FakeK8sClient) PodByID(ctx context.Context, pID PodID, n Namespace) (*v1.Pod, error) {
	return nil, nil
}

func FakePodStatus(image reference.NamedTagged, phase string) v1.PodStatus {
	return v1.PodStatus{
		Phase: v1.PodPhase(phase),
		ContainerStatuses: []v1.ContainerStatus{
			{
				Name:        "main",
				ContainerID: "docker://" + MagicTestContainerID,
				Image:       image.String(),
				Ready:       true,
			},
			{
				Name:        "tilt-synclet",
				ContainerID: "docker://tilt-testsynclet",
				// can't use the constants in synclet because that would create a dep cycle
				Image: "gcr.io/windmill-public-containers/tilt-synclet:latest",
				Ready: true,
			},
		},
	}
}

func FakePodSpec(image reference.NamedTagged) v1.PodSpec {
	return v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "main",
				Image: image.String(),
				Ports: []v1.ContainerPort{
					{
						ContainerPort: 8080,
					},
				},
			},
			{
				Name:  "tilt-synclet",
				Image: "gcr.io/windmill-public-containers/tilt-synclet:latest",
			},
		},
	}
}

func (c *FakeK8sClient) applyWasCalled() bool {
	return c.Yaml != ""
}

func (c *FakeK8sClient) ForwardPort(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int) (int, func(), error) {
	c.LastForwardPortPodID = podID
	c.LastForwardPortRemotePort = remotePort
	return optionalLocalPort, func() {}, nil
}

func (c *FakeK8sClient) ContainerRuntime(ctx context.Context) container.Runtime {
	if c.Runtime != "" {
		return c.Runtime
	}
	return container.RuntimeDocker
}

func (c *FakeK8sClient) PrivateRegistry(ctx context.Context) container.Registry {
	return c.Registry
}

func (c *FakeK8sClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return nil
}

type BufferCloser struct {
	*bytes.Buffer
}

func (b BufferCloser) Close() error {
	return nil
}

var _ io.ReadCloser = BufferCloser{}
