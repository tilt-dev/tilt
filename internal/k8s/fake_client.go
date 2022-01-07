package k8s

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// A magic constant. If the docker client returns this constant, we always match
// even if the container doesn't have the correct image name.
const MagicTestContainerID = "tilt-testcontainer"

// MagicTestExplodingPort causes FakePortForwarder to fail to initialize (i.e. return an error as soon as ForwardPorts
// is called without ever becoming ready).
const MagicTestExplodingPort = 34743

var _ Client = &FakeK8sClient{}

// For keying PodLogsByPodAndContainer
type PodAndCName struct {
	PID   PodID
	CName container.Name
}

type FakeK8sClient struct {
	t            testing.TB
	mu           sync.Mutex
	ownerFetcher OwnerFetcher

	FakePortForwardClient

	Yaml string
	Lb   LoadBalancerSpec

	DeletedYaml string
	DeleteError error

	LastPodQueryNamespace Namespace
	LastPodQueryImage     reference.NamedTagged

	PodLogsByPodAndContainer map[PodAndCName]ReaderCloser
	lastPodLogStartTime      time.Time
	lastPodLogContext        context.Context
	LastPodLogPipeWriter     *io.PipeWriter
	ContainerLogsError       error

	podWatches     []fakePodWatch
	serviceWatches []fakeServiceWatch
	eventWatches   []fakeEventWatch
	events         map[types.NamespacedName]*v1.Event
	services       map[types.NamespacedName]*v1.Service
	pods           map[types.NamespacedName]*v1.Pod

	EventsWatchErr error

	UpsertError      error
	UpsertResult     []K8sEntity
	LastUpsertResult []K8sEntity
	UpsertTimeout    time.Duration

	Runtime    container.Runtime
	Registry   container.Registry
	FakeNodeIP NodeIP

	// entities are injected objects keyed by UID.
	entities map[types.UID]K8sEntity
	// currentVersions maintains a mapping of object name to UID which represents the most recently injected value.
	//
	// In real K8s, you'd need to delete the old object before being able to store the new one with the same name.
	// For testing purposes, it's useful to be able to simulate out of order/stale data type scenarios, so the fake
	// client doesn't enforce name uniqueness for storage. When appropriate (e.g. ListMeta), this map ensures that
	// multiple objects for the same name aren't returned.
	currentVersions         map[string]types.UID
	getByReferenceCallCount int
	listCallCount           int
	listReturnsEmpty        bool

	ExecCalls   []ExecCall
	ExecOutputs []io.Reader
	ExecErrors  []error
}

var _ Client = &FakeK8sClient{}

type ExecCall struct {
	PID   PodID
	CName container.Name
	Ns    Namespace
	Cmd   []string
	Stdin []byte
}

type fakeServiceWatch struct {
	cancel func()
	ns     Namespace
	ch     chan *v1.Service
}

type fakePodWatch struct {
	cancel func()
	ns     Namespace
	ch     chan ObjectUpdate
}

type fakeEventWatch struct {
	cancel func()
	ns     Namespace
	ch     chan *v1.Event
}

func (c *FakeK8sClient) UpsertService(s *v1.Service) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s = s.DeepCopy()
	c.services[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
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

	pod = pod.DeepCopy()
	c.pods[types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}] = pod
	for _, w := range c.podWatches {
		if w.ns != Namespace(pod.Namespace) {
			continue
		}

		w.ch <- ObjectUpdate{obj: pod}
	}
}

func (c *FakeK8sClient) UpsertEvent(event *v1.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	event = event.DeepCopy()
	c.events[types.NamespacedName{Name: event.Name, Namespace: event.Namespace}] = event
	for _, w := range c.eventWatches {
		if w.ns != Namespace(event.Namespace) {
			continue
		}

		w.ch <- event
	}
}

func (c *FakeK8sClient) PodFromInformerCache(ctx context.Context, nn types.NamespacedName) (*v1.Pod, error) {
	if nn.Namespace == "" {
		return nil, fmt.Errorf("missing namespace from pod request")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	pod, ok := c.pods[nn]
	if !ok {
		return nil, apierrors.NewNotFound(PodGVR.GroupResource(), nn.Name)
	}
	return pod, nil
}

func (c *FakeK8sClient) WatchServices(ctx context.Context, ns Namespace) (<-chan *v1.Service, error) {
	if ns == "" {
		return nil, fmt.Errorf("missing namespace from watch request")
	}

	ctx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	ch := make(chan *v1.Service, 20)
	c.serviceWatches = append(c.serviceWatches, fakeServiceWatch{cancel, ns, ch})
	toEmit := []*v1.Service{}
	for _, service := range c.services {
		if Namespace(service.Namespace) == ns {
			toEmit = append(toEmit, service)
		}
	}
	c.mu.Unlock()

	go func() {
		// Initial list of objects
		for _, obj := range toEmit {
			ch <- obj
		}

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

		close(ch)
	}()
	return ch, nil
}

func (c *FakeK8sClient) WatchEvents(ctx context.Context, ns Namespace) (<-chan *v1.Event, error) {
	if ns == "" {
		return nil, fmt.Errorf("missing namespace from watch request")
	}

	if c.EventsWatchErr != nil {
		err := c.EventsWatchErr
		c.EventsWatchErr = nil
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	ch := make(chan *v1.Event, 20)
	c.eventWatches = append(c.eventWatches, fakeEventWatch{cancel, ns, ch})
	toEmit := []*v1.Event{}
	for _, event := range c.events {
		if Namespace(event.Namespace) == ns {
			toEmit = append(toEmit, event)
		}
	}
	c.mu.Unlock()

	go func() {
		// Initial list of objects
		for _, obj := range toEmit {
			ch <- obj
		}

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

		close(ch)
	}()
	return ch, nil
}

func (c *FakeK8sClient) WatchMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) (<-chan metav1.Object, error) {
	if ns == "" {
		return nil, fmt.Errorf("missing namespace from watch request")
	}

	return make(chan metav1.Object), nil
}

func (c *FakeK8sClient) EmitPodDelete(p *v1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.pods, types.NamespacedName{Name: p.Name, Namespace: p.Namespace})
	for _, w := range c.podWatches {
		if w.ns != Namespace(p.Namespace) {
			continue
		}

		w.ch <- ObjectUpdate{obj: p, isDelete: true}
	}
}

func (c *FakeK8sClient) WatchPods(ctx context.Context, ns Namespace) (<-chan ObjectUpdate, error) {
	if ns == "" {
		return nil, fmt.Errorf("missing namespace from watch request")
	}

	ctx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	ch := make(chan ObjectUpdate, 20)
	c.podWatches = append(c.podWatches, fakePodWatch{cancel, ns, ch})
	toEmit := []*v1.Pod{}
	for _, pod := range c.pods {
		if Namespace(pod.Namespace) == ns {
			toEmit = append(toEmit, pod)
		}
	}
	c.mu.Unlock()

	go func() {
		// Initial list of objects
		for _, obj := range toEmit {
			ch <- ObjectUpdate{obj: obj}
		}

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

		close(ch)
	}()

	return ch, nil
}

func NewFakeK8sClient(t testing.TB) *FakeK8sClient {
	cli := &FakeK8sClient{
		t:                        t,
		PodLogsByPodAndContainer: make(map[PodAndCName]ReaderCloser),
		pods:                     make(map[types.NamespacedName]*v1.Pod),
		services:                 make(map[types.NamespacedName]*v1.Service),
		events:                   make(map[types.NamespacedName]*v1.Event),
		entities:                 make(map[types.UID]K8sEntity),
		currentVersions:          make(map[string]types.UID),
	}
	ctx, cancel := context.WithCancel(logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout)))
	t.Cleanup(cancel)
	cli.ownerFetcher = NewOwnerFetcher(ctx, cli)
	return cli
}

func (c *FakeK8sClient) TearDown() {
	c.mu.Lock()
	podWatches := append([]fakePodWatch{}, c.podWatches...)
	serviceWatches := append([]fakeServiceWatch{}, c.serviceWatches...)
	eventWatches := append([]fakeEventWatch{}, c.eventWatches...)
	c.mu.Unlock()

	for _, watch := range podWatches {
		watch.cancel()
		for range watch.ch {
		}
	}
	for _, watch := range serviceWatches {
		watch.cancel()
		for range watch.ch {
		}
	}
	for _, watch := range eventWatches {
		watch.cancel()
		for range watch.ch {
		}
	}
}

func (c *FakeK8sClient) Upsert(_ context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.UpsertError != nil {
		return nil, c.UpsertError
	}

	var result []K8sEntity
	if c.UpsertResult != nil {
		result = c.UpsertResult
	} else {
		yaml, err := SerializeSpecYAML(entities)
		if err != nil {
			return nil, errors.Wrap(err, "kubectl apply")
		}
		c.Yaml = yaml

		for _, e := range entities {
			clone := e.DeepCopy()
			clone.SetUID(uuid.New().String())
			result = append(result, clone)
		}
	}

	c.LastUpsertResult = result
	c.UpsertTimeout = timeout

	return result, nil
}

func (c *FakeK8sClient) Delete(_ context.Context, entities []K8sEntity, wait bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

// Inject adds an entity or replaces it for subsequent retrieval.
//
// Entities are keyed by UID.
func (c *FakeK8sClient) Inject(entities ...K8sEntity) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t.Helper()
	for i, entity := range entities {
		if entity.UID() == "" {
			c.t.Fatalf("Entity with name[%s] at index[%d] had no UID", entity.Name(), i)
		}
		c.entities[entity.UID()] = entity
		c.currentVersions[entity.Name()] = entity.UID()
	}
}

func (c *FakeK8sClient) GetMetaByReference(ctx context.Context, ref v1.ObjectReference) (metav1.Object, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getByReferenceCallCount++
	resp, ok := c.entities[ref.UID]
	if !ok {
		logger.Get(ctx).Infof("FakeK8sClient.GetMetaByReference: resource not found: %s", ref.Name)
		return nil, apierrors.NewNotFound(v1.Resource(ref.Kind), ref.Name)
	}
	return resp.Meta(), nil
}

func (c *FakeK8sClient) ListMeta(_ context.Context, gvk schema.GroupVersionKind, ns Namespace) ([]metav1.Object, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.listCallCount++
	if c.listReturnsEmpty {
		return nil, nil
	}

	result := make([]metav1.Object, 0)
	for _, uid := range c.currentVersions {
		entity := c.entities[uid]
		if entity.Namespace().String() != ns.String() {
			continue
		}
		if entity.GVK() != gvk {
			continue
		}
		result = append(result, entity.Meta())
	}
	return result, nil
}

func (c *FakeK8sClient) SetLogsForPodContainer(pID PodID, cName container.Name, logs string) {

	c.SetLogReaderForPodContainer(pID, cName, strings.NewReader(logs))
}

func (c *FakeK8sClient) SetLogReaderForPodContainer(pID PodID, cName container.Name, reader io.Reader) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.PodLogsByPodAndContainer[PodAndCName{pID, cName}] = ReaderCloser{Reader: reader}
}

func (c *FakeK8sClient) ContainerLogs(ctx context.Context, pID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ContainerLogsError != nil {
		return nil, c.ContainerLogsError
	}

	// metav1.Time truncates to the nearest second when serializing across the
	// wire, so truncate here to replicate that behavior.
	c.lastPodLogStartTime = startTime.Truncate(time.Second)
	c.lastPodLogContext = ctx

	// If we have specific logs for this pod/container combo, return those
	if buf, ok := c.PodLogsByPodAndContainer[PodAndCName{pID, cName}]; ok {
		return buf, nil
	}

	r, w := io.Pipe()
	c.LastPodLogPipeWriter = w

	return ReaderCloser{Reader: r}, nil
}

func (c *FakeK8sClient) LastPodLogStartTime() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.lastPodLogStartTime
}

func (c *FakeK8sClient) LastPodLogContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.lastPodLogContext
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Runtime != "" {
		return c.Runtime
	}
	return container.RuntimeDocker
}

func (c *FakeK8sClient) LocalRegistry(ctx context.Context) container.Registry {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Registry
}

func (c *FakeK8sClient) NodeIP(ctx context.Context) NodeIP {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.FakeNodeIP
}

func (c *FakeK8sClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *FakeK8sClient) CheckConnected(ctx context.Context) (*version.Info, error) {
	return &version.Info{}, nil
}

func (c *FakeK8sClient) OwnerFetcher() OwnerFetcher {
	return c.ownerFetcher
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
	namespace Namespace
	ctx       context.Context
	ready     chan struct{}
	done      chan error
}

var _ PortForwarder = FakePortForwarder{}

func NewFakePortForwarder(ctx context.Context, localPort int, namespace Namespace) FakePortForwarder {
	return FakePortForwarder{
		localPort: localPort,
		namespace: namespace,
		ctx:       ctx,
		ready:     make(chan struct{}, 1),
		done:      make(chan error),
	}
}

func (pf FakePortForwarder) Addresses() []string {
	return []string{"127.0.0.1", "::1"}
}

func (pf FakePortForwarder) LocalPort() int {
	return pf.localPort
}
func (pf FakePortForwarder) Namespace() Namespace {
	return pf.namespace
}

func (pf FakePortForwarder) ForwardPorts() error {
	// in the real port forwarder, the binding/listening logic can fail before the forwarder signals it's ready
	// to simulate this in tests, there's a magic port number
	if pf.localPort == MagicTestExplodingPort {
		return errors.New("fake error starting port forwarding")
	}

	close(pf.ready)

	select {
	case <-pf.ctx.Done():
		// NOTE: the context error should NOT be returned here
		return nil
	case err := <-pf.done:
		return err
	}
}

func (pf FakePortForwarder) ReadyCh() <-chan struct{} {
	return pf.ready
}

// TriggerFailure allows tests to inject errors during forwarding that will be returned by ForwardPorts.
func (pf FakePortForwarder) TriggerFailure(err error) {
	pf.done <- err
}

type FakePortForwardClient struct {
	mu               sync.Mutex
	portForwardCalls []PortForwardCall
}

func NewFakePortForwardClient() *FakePortForwardClient {
	return &FakePortForwardClient{
		portForwardCalls: []PortForwardCall{},
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
	c.mu.Lock()
	defer c.mu.Unlock()

	result := NewFakePortForwarder(ctx, optionalLocalPort, namespace)
	c.portForwardCalls = append(c.portForwardCalls, PortForwardCall{
		PodID:      podID,
		RemotePort: remotePort,
		Host:       host,
		Forwarder:  result,
		Context:    ctx,
	})

	return result, nil
}

func (c *FakePortForwardClient) CreatePortForwardCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.portForwardCalls)
}
func (c *FakePortForwardClient) LastForwardPortPodID() PodID {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.portForwardCalls) == 0 {
		return ""
	}
	return c.portForwardCalls[len(c.portForwardCalls)-1].PodID
}
func (c *FakePortForwardClient) LastForwardPortRemotePort() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.portForwardCalls) == 0 {
		return 0
	}
	return c.portForwardCalls[len(c.portForwardCalls)-1].RemotePort
}
func (c *FakePortForwardClient) LastForwardPortHost() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.portForwardCalls) == 0 {
		return ""
	}
	return c.portForwardCalls[len(c.portForwardCalls)-1].Host
}
func (c *FakePortForwardClient) LastForwarder() FakePortForwarder {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.portForwardCalls) == 0 {
		return FakePortForwarder{}
	}
	return c.portForwardCalls[len(c.portForwardCalls)-1].Forwarder
}
func (c *FakePortForwardClient) LastForwardContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.portForwardCalls) == 0 {
		return nil
	}
	return c.portForwardCalls[len(c.portForwardCalls)-1].Context
}
func (c *FakePortForwardClient) PortForwardCalls() []PortForwardCall {
	c.mu.Lock()
	defer c.mu.Unlock()

	calls := make([]PortForwardCall, len(c.portForwardCalls))
	for i, call := range c.portForwardCalls {
		calls[i] = call
	}
	return calls
}
