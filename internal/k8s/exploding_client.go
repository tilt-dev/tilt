package k8s

import (
	"context"
	"io"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ Client = &explodingClient{}

type explodingClient struct {
	err error
}

func (ec *explodingClient) Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) Delete(ctx context.Context, entities []K8sEntity) error {
	return errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) GetMetaByReference(ctx context.Context, ref v1.ObjectReference) (ObjectMeta, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) ListMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) ([]ObjectMeta, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) PodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, lp []model.LabelPair) ([]v1.Pod, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) PollForPodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, lp []model.LabelPair, timeout time.Duration) ([]v1.Pod, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) PodByID(ctx context.Context, podID PodID, n Namespace) (*v1.Pod, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) ContainerLogs(ctx context.Context, podID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) WatchPods(ctx context.Context, ns Namespace) (<-chan ObjectUpdate, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) WatchServices(ctx context.Context, ns Namespace) (<-chan *v1.Service, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) WatchEvents(ctx context.Context, ns Namespace) (<-chan *v1.Event, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) WatchMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) (<-chan ObjectMeta, error) {
	return nil, errors.Wrap(ec.err, "could not set up k8s client")
}

func (ec *explodingClient) ContainerRuntime(ctx context.Context) container.Runtime {
	return container.RuntimeUnknown
}

func (ec *explodingClient) LocalRegistry(ctx context.Context) container.Registry {
	return container.Registry{}
}

func (ec *explodingClient) NodeIP(ctx context.Context) NodeIP {
	return ""
}

func (ec *explodingClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return errors.Wrap(ec.err, "could not set up k8s client")
}
