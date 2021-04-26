package k8s

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"
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
	mu sync.Mutex

	FakePortForwardClient

	Yaml string
	Lb   LoadBalancerSpec

	DeletedYaml string
	DeleteError error

	LastPodQueryNamespace Namespace
	LastPodQueryImage     reference.NamedTagged

	PodLogsByPodAndContainer map[PodAndCName]ReaderCloser
	LastPodLogStartTime      time.Time
	LastPodLogContext        context.Context
	ContainerLogsError       error

	podWatches     []fakePodWatch
	serviceWatches []fakeServiceWatch
	eventWatches   []fakeEventWatch
	pods           map[types.NamespacedName]*v1.Pod

	EventsWatchErr error

	UpsertError      error
	LastUpsertResult []K8sEntity
	UpsertTimeout    time.Duration

	Runtime    container.Runtime
	Registry   container.Registry
	FakeNodeIP NodeIP

	entityByName            map[string]K8sEntity
	getByReferenceCallCount int
	listCallCount           int
	listReturnsEmpty        bool

	ExecCalls   []ExecCall
	ExecOutputs []io.Reader
	ExecErrors  []error
}

type ExecCall struct {
	PID   PodID
	CName container.Name
	Ns    Namespace
	Cmd   []string
	Stdin []byte
}

type fakeServiceWatch struct {
	ns Namespace
	ch chan *v1.Service
}

type fakePodWatch struct {
	ns Namespace
	ch chan ObjectUpdate
}

type fakeEventWatch struct {
	ns Namespace
	ch chan *v1.Event
}

func (c *FakeK8sClient) EmitService(ls labels.Selector, s *v1.Service) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, w := range c.serviceWatches {
		if w.ns != Namespace(s.Namespace) {
			continue
		}

		w.ch <- s
	}
}

func (c *FakeK8sClient) UpsertPod(pod *v1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pods[types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}] = pod
}

func (c *FakeK8sClient) PodFromInformerCache(ctx context.Context, nn types.NamespacedName) (*v1.Pod, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pod, ok := c.pods[nn]
	if !ok {
		return nil, apierrors.NewNotFound(PodGVR.GroupResource(), nn.Name)
	}
	return pod, nil
}

func (c *FakeK8sClient) WatchServices(ctx context.Context, ns Namespace) (<-chan *v1.Service, error) {
	c.mu.Lock()
	ch := make(chan *v1.Service, 20)
	c.serviceWatches = append(c.serviceWatches, fakeServiceWatch{ns, ch})
	c.mu.Unlock()

	go func() {
		// when ctx is canceled, remove the label selector from the list of watched label selectors
		<-ctx.Done()
		c.mu.Lock()
		var newWatches []fakeServiceWatch
		for _, e := range c.serviceWatches {
			if e.ns != ns {
				newWatches = append(newWatches, e)
			}
		}
		c.serviceWatches = newWatches
		c.mu.Unlock()
	}()
	return ch, nil
}

func (c *FakeK8sClient) WatchEvents(ctx context.Context, ns Namespace) (<-chan *v1.Event, error) {
	if c.EventsWatchErr != nil {
		err := c.EventsWatchErr
		c.EventsWatchErr = nil
		return nil, err
	}

	c.mu.Lock()
	ch := make(chan *v1.Event, 20)
	c.eventWatches = append(c.eventWatches, fakeEventWatch{ns, ch})
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		c.mu.Lock()
		var newWatches []fakeEventWatch
		for _, e := range c.eventWatches {
			if e.ns != ns {
				newWatches = append(newWatches, e)
			}
		}
		c.eventWatches = newWatches
		c.mu.Unlock()
	}()
	return ch, nil
}

func (c *FakeK8sClient) WatchMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) (<-chan ObjectMeta, error) {
	return make(chan ObjectMeta), nil
}

func (c *FakeK8sClient) EmitEvent(ctx context.Context, evt *v1.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, w := range c.eventWatches {
		if w.ns != "" && w.ns != Namespace(evt.Namespace) {
			continue
		}

		w.ch <- evt
	}
}

func (c *FakeK8sClient) EmitPod(ls labels.Selector, p *v1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, w := range c.podWatches {
		if w.ns != Namespace(p.Namespace) {
			continue
		}

		w.ch <- ObjectUpdate{obj: p}
	}
}

func (c *FakeK8sClient) EmitPodDelete(ls labels.Selector, p *v1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, w := range c.podWatches {
		if w.ns != Namespace(p.Namespace) {
			continue
		}

		w.ch <- ObjectUpdate{obj: p, isDelete: true}
	}
}

func (c *FakeK8sClient) WatchPods(ctx context.Context, ns Namespace) (<-chan ObjectUpdate, error) {
	c.mu.Lock()
	ch := make(chan ObjectUpdate, 20)
	c.podWatches = append(c.podWatches, fakePodWatch{ns, ch})
	c.mu.Unlock()

	go func() {
		// when ctx is canceled, remove the label selector from the list of watched label selectors
		<-ctx.Done()
		c.mu.Lock()
		var newWatches []fakePodWatch
		for _, e := range c.podWatches {
			if e.ns != ns {
				newWatches = append(newWatches, e)
			}
		}
		c.podWatches = newWatches
		c.mu.Unlock()
	}()
	return ch, nil
}

func NewFakeK8sClient() *FakeK8sClient {
	return &FakeK8sClient{
		PodLogsByPodAndContainer: make(map[PodAndCName]ReaderCloser),
		pods:                     make(map[types.NamespacedName]*v1.Pod),
	}
}

func (c *FakeK8sClient) TearDown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, watch := range c.podWatches {
		close(watch.ch)
	}
	for _, watch := range c.serviceWatches {
		close(watch.ch)
	}
	for _, watch := range c.eventWatches {
		close(watch.ch)
	}
}

func (c *FakeK8sClient) Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error) {
	if c.UpsertError != nil {
		return nil, c.UpsertError
	}
	yaml, err := SerializeSpecYAML(entities)
	if err != nil {
		return nil, errors.Wrap(err, "kubectl apply")
	}
	c.Yaml = yaml

	result := make([]K8sEntity, 0, len(entities))

	for _, e := range entities {
		clone := e.DeepCopy()
		err = SetUID(&clone, uuid.New().String())
		if err != nil {
			return nil, errors.Wrap(err, "Upsert: generating UUID")
		}
		result = append(result, clone)
	}

	c.LastUpsertResult = result
	c.UpsertTimeout = timeout
	return result, nil
}

func (c *FakeK8sClient) Delete(ctx context.Context, entities []K8sEntity) error {
	if c.DeleteError != nil {
		err := c.DeleteError
		c.DeleteError = nil
		return err
	}

	yaml, err := SerializeSpecYAML(entities)
	if err != nil {
		return errors.Wrap(err, "kubectl delete")
	}
	c.DeletedYaml = yaml
	return nil
}

func (c *FakeK8sClient) InjectEntityByName(entities ...K8sEntity) {
	if c.entityByName == nil {
		c.entityByName = make(map[string]K8sEntity)
	}
	for _, entity := range entities {
		c.entityByName[entity.Name()] = entity
	}
}

func (c *FakeK8sClient) GetMetaByReference(ctx context.Context, ref v1.ObjectReference) (ObjectMeta, error) {
	c.getByReferenceCallCount++
	resp, ok := c.entityByName[ref.Name]
	if !ok {
		logger.Get(ctx).Infof("FakeK8sClient.GetMetaByReference: resource not found: %s", ref.Name)
		return nil, apierrors.NewNotFound(v1.Resource(ref.Kind), ref.Name)
	}
	return resp.meta(), nil
}

func (c *FakeK8sClient) ListMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) ([]ObjectMeta, error) {
	c.listCallCount++
	if c.listReturnsEmpty {
		return nil, nil
	}

	result := make([]ObjectMeta, 0)
	for _, entity := range c.entityByName {
		if entity.Namespace() != ns {
			continue
		}
		if entity.GVK() != gvk {
			continue
		}
		result = append(result, entity.meta())
	}
	return result, nil
}

func (c *FakeK8sClient) SetLogsForPodContainer(pID PodID, cName container.Name, logs string) {
	c.SetLogReaderForPodContainer(pID, cName, strings.NewReader(logs))
}

func (c *FakeK8sClient) SetLogReaderForPodContainer(pID PodID, cName container.Name, reader io.Reader) {
	c.PodLogsByPodAndContainer[PodAndCName{pID, cName}] = ReaderCloser{Reader: reader}
}

func (c *FakeK8sClient) ContainerLogs(ctx context.Context, pID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error) {
	if c.ContainerLogsError != nil {
		return nil, c.ContainerLogsError
	}

	// metav1.Time truncates to the nearest second when serializing across the
	// wire, so truncate here to replicate that behavior.
	c.LastPodLogStartTime = startTime.Truncate(time.Second)
	c.LastPodLogContext = ctx

	// If we have specific logs for this pod/container combo, return those
	if buf, ok := c.PodLogsByPodAndContainer[PodAndCName{pID, cName}]; ok {
		return buf, nil
	}

	return ReaderCloser{Reader: bytes.NewBuffer(nil)}, nil
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

func (c *FakeK8sClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error) {
	pfc := &(c.FakePortForwardClient)
	return pfc.CreatePortForwarder(ctx, namespace, podID, optionalLocalPort, remotePort, host)
}

func (c *FakeK8sClient) ContainerRuntime(ctx context.Context) container.Runtime {
	if c.Runtime != "" {
		return c.Runtime
	}
	return container.RuntimeDocker
}

func (c *FakeK8sClient) LocalRegistry(ctx context.Context) container.Registry {
	return c.Registry
}

func (c *FakeK8sClient) NodeIP(ctx context.Context) NodeIP {
	return c.FakeNodeIP
}

func (c *FakeK8sClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	var stdinBytes []byte
	var err error
	if stdin != nil {
		stdinBytes, err = ioutil.ReadAll(stdin)
		if err != nil {
			return errors.Wrap(err, "reading Exec stdin")
		}
	}

	c.ExecCalls = append(c.ExecCalls, ExecCall{
		PID:   podID,
		CName: cName,
		Ns:    n,
		Cmd:   cmd,
		Stdin: stdinBytes,
	})

	if len(c.ExecOutputs) > 0 {
		out := c.ExecOutputs[0]
		c.ExecOutputs = c.ExecOutputs[1:]
		_, _ = io.Copy(stdout, out)
	}

	if len(c.ExecErrors) > 0 {
		err = c.ExecErrors[0]
		c.ExecErrors = c.ExecErrors[1:]
		return err
	}
	return nil
}

type ReaderCloser struct {
	io.Reader
}

func (b ReaderCloser) Close() error {
	return nil
}

var _ io.ReadCloser = ReaderCloser{}

type FakePortForwarder struct {
	localPort int
	ctx       context.Context
	Done      chan error
}

func (pf FakePortForwarder) LocalPort() int {
	return pf.localPort
}

func (pf FakePortForwarder) ForwardPorts() error {
	select {
	case <-pf.ctx.Done():
		return pf.ctx.Err()
	case <-pf.Done:
		return nil
	}
}

type FakePortForwardClient struct {
	PortForwardCalls []PortForwardCall
}

func NewFakePortfowardClient() *FakePortForwardClient {
	return &FakePortForwardClient{
		PortForwardCalls: []PortForwardCall{},
	}
}

type PortForwardCall struct {
	PodID      PodID
	RemotePort int
	Host       string
	Forwarder  FakePortForwarder
	Context    context.Context
}

func (c *FakePortForwardClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error) {
	result := FakePortForwarder{
		localPort: optionalLocalPort,
		ctx:       ctx,
		Done:      make(chan error),
	}

	c.PortForwardCalls = append(c.PortForwardCalls, PortForwardCall{
		PodID:      podID,
		RemotePort: remotePort,
		Host:       host,
		Forwarder:  result,
		Context:    ctx,
	})

	return result, nil
}

func (c *FakePortForwardClient) CreatePortForwardCallCount() int {
	return len(c.PortForwardCalls)
}
func (c *FakePortForwardClient) LastForwardPortPodID() PodID {
	return c.PortForwardCalls[len(c.PortForwardCalls)-1].PodID
}
func (c *FakePortForwardClient) LastForwardPortRemotePort() int {
	return c.PortForwardCalls[len(c.PortForwardCalls)-1].RemotePort
}
func (c *FakePortForwardClient) LastForwardPortHost() string {
	return c.PortForwardCalls[len(c.PortForwardCalls)-1].Host
}
func (c *FakePortForwardClient) LastForwarder() FakePortForwarder {
	return c.PortForwardCalls[len(c.PortForwardCalls)-1].Forwarder
}
func (c *FakePortForwardClient) LastForwardContext() context.Context {
	return c.PortForwardCalls[len(c.PortForwardCalls)-1].Context
}
