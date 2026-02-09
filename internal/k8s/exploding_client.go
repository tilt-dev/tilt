package k8s

import (
	"context"
	"io"
	"time"

	"github.com/distribution/reference"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ Client = &explodingClient{}

type explodingClient struct {
	err error
}

func NewExplodingClient(err error) Client {
	return &explodingClient{
		err: err,
	}
}

func (ec *explodingClient) Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration, ssa SSAOptions) ([]K8sEntity, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) Delete(ctx context.Context, entities []K8sEntity, wait time.Duration) error {
	return errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) GetMetaByReference(ctx context.Context, ref v1.ObjectReference) (metav1.Object, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) ListMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) ([]metav1.Object, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) PodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, lp []model.LabelPair) ([]v1.Pod, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) PollForPodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, lp []model.LabelPair, timeout time.Duration) ([]v1.Pod, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) ContainerLogs(ctx context.Context, podID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) WatchPods(ctx context.Context, ns Namespace) (<-chan ObjectUpdate, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) PodFromInformerCache(ctx context.Context, nn types.NamespacedName) (*v1.Pod, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) WatchServices(ctx context.Context, ns Namespace) (<-chan *v1.Service, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) WatchEvents(ctx context.Context, ns Namespace) (<-chan *v1.Event, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) WatchMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) (<-chan metav1.Object, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) ContainerRuntime(ctx context.Context) container.Runtime {
	return container.RuntimeUnknown
}

func (ec *explodingClient) LocalRegistry(_ context.Context) *v1alpha1.RegistryHosting {
	return nil
}

func (ec *explodingClient) NodeIP(ctx context.Context) NodeIP {
	return ""
}

func (ec *explodingClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) CheckConnected(ctx context.Context) (*version.Info, error) {
	return nil, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) OwnerFetcher() OwnerFetcher {
	return NewOwnerFetcher(context.Background(), ec)
}

func (ec *explodingClient) ClusterHealth(_ context.Context, _ bool) (ClusterHealth, error) {
	return ClusterHealth{}, errors.Wrap(ec.err, "could not set up kubernetes client")
}

func (ec *explodingClient) APIConfig() *api.Config {
	return &api.Config{}
}
