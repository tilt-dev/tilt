package engine

import (
	"context"
	"fmt"
	"sync"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/options"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet"
	"google.golang.org/grpc"
)

type newCliFn func(ctx context.Context, kCli k8s.Client, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error)
type SyncletManager struct {
	kCli      k8s.Client
	mutex     *sync.Mutex
	clients   map[k8s.PodID]synclet.SyncletClient
	newClient newCliFn
}

type tunneledSyncletClient struct {
	synclet.SyncletClient
	tunnelCloser func()
}

var _ synclet.SyncletClient = tunneledSyncletClient{}

func (t tunneledSyncletClient) Close() error {
	err := t.SyncletClient.Close()
	if err != nil {
		return err
	}

	t.tunnelCloser()

	return nil
}

func NewSyncletManager(kCli k8s.Client) SyncletManager {
	return SyncletManager{
		kCli:      kCli,
		mutex:     new(sync.Mutex),
		clients:   make(map[k8s.PodID]synclet.SyncletClient),
		newClient: newSyncletClient,
	}
}

func NewSyncletManagerForTests(kCli k8s.Client, fakeCli synclet.SyncletClient) SyncletManager {
	newClientFn := func(ctx context.Context, kCli k8s.Client, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error) {
		fake, ok := fakeCli.(*synclet.FakeSyncletClient)
		if ok {
			fake.PodID = podID
			fake.Namespace = ns
		}
		return fakeCli, nil
	}

	return SyncletManager{
		kCli:      kCli,
		mutex:     new(sync.Mutex),
		clients:   make(map[k8s.PodID]synclet.SyncletClient),
		newClient: newClientFn,
	}
}

func (sm SyncletManager) ClientForPod(ctx context.Context, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	client, ok := sm.clients[podID]
	if ok {
		return client, nil
	}

	client, err := sm.newClient(ctx, sm.kCli, podID, ns)
	if err != nil {
		return nil, errors.Wrap(err, "error creating synclet client")
	}
	sm.clients[podID] = client

	return client, nil
}

func (sm SyncletManager) ForgetPod(ctx context.Context, podID k8s.PodID) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	client, ok := sm.clients[podID]
	if !ok {
		// if we don't know about the pod, it's already forgotten - noop
		return nil
	}

	delete(sm.clients, podID)

	return client.Close()
}

func newSyncletClient(ctx context.Context, kCli k8s.Client, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SidecarSyncletManager-newSidecarSyncletClient")
	defer span.Finish()

	pod, err := kCli.PodByID(ctx, podID, ns)
	if err != nil {
		return nil, errors.Wrap(err, "newSyncletClient")
	}

	// Make sure that the synclet container is ready and not crashlooping.
	_, err = k8s.WaitForContainerReady(ctx, kCli, pod, sidecar.SyncletImageRef)
	if err != nil {
		return nil, errors.Wrap(err, "newSyncletClient")
	}

	// TODO(nick): We need a better way to kill the client when the pod dies.
	tunneledPort, tunnelCloser, err := kCli.ForwardPort(ctx, ns, podID, synclet.Port)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening tunnel to synclet pod '%s'", podID)
	}

	logger.Get(ctx).Verbosef("tunneling to synclet client at %s (local port %d)", podID.String(), tunneledPort)

	t := opentracing.GlobalTracer()

	opts := options.MaxMsgDial()
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, options.TracingInterceptorsDial(t)...)

	conn, err := grpc.DialContext(ctx, fmt.Sprintf("127.0.0.1:%d", tunneledPort), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to synclet")
	}

	return tunneledSyncletClient{synclet.NewGRPCClient(conn), tunnelCloser}, nil
}
