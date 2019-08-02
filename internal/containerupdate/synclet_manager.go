package containerupdate

import (
	"context"
	"fmt"
	"sync"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/options"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet"
)

type newCliFn func(ctx context.Context, kCli k8s.Client, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error)
type SyncletManager struct {
	kCli      k8s.Client
	mutex     *sync.Mutex
	clients   map[k8s.PodID]synclet.SyncletClient
	newClient newCliFn

	// Ensures that we don't try to setup a client multiple times if it keeps failing.
	clientWarmAttempted map[k8s.PodID]bool
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
		kCli:                kCli,
		mutex:               new(sync.Mutex),
		clients:             make(map[k8s.PodID]synclet.SyncletClient),
		clientWarmAttempted: make(map[k8s.PodID]bool),
		newClient:           newSyncletClient,
	}
}

func NewSyncletManagerForTests(kCli k8s.Client, fakeCli synclet.SyncletClient) SyncletManager {
	newClientFn := func(ctx context.Context, kCli k8s.Client, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error) {
		fake, ok := fakeCli.(*synclet.TestSyncletClient)
		if ok {
			fake.PodID = podID
			fake.Namespace = ns
		}
		return fakeCli, nil
	}

	return SyncletManager{
		kCli:                kCli,
		mutex:               new(sync.Mutex),
		clients:             make(map[k8s.PodID]synclet.SyncletClient),
		clientWarmAttempted: make(map[k8s.PodID]bool),
		newClient:           newClientFn,
	}
}

type syncletEntry struct {
	PodID     k8s.PodID
	Namespace k8s.Namespace
}

func (sm SyncletManager) diff(ctx context.Context, st store.RStore) (setup []syncletEntry, teardown []k8s.PodID) {
	state := st.RLockState()
	defer st.RUnlockState()

	// We don't need synclets if we're not watching the FS for changes.
	if !state.WatchFiles {
		return
	}

	activePodIDs := make(map[k8s.PodID]bool)

	// Look for all the pods that have synclets, and
	// start warming the connection.
	for _, ms := range state.ManifestStates() {
		for _, pod := range ms.PodSet.Pods {
			if !pod.HasSynclet {
				continue
			}

			id := pod.PodID
			activePodIDs[id] = true
			_, hasClient := sm.clients[id]
			if hasClient || sm.clientWarmAttempted[id] {
				continue
			}

			sm.clientWarmAttempted[id] = true
			setup = append(setup, syncletEntry{
				PodID:     pod.PodID,
				Namespace: pod.Namespace,
			})
		}
	}

	for podID := range sm.clients {
		if !activePodIDs[podID] {
			teardown = append(teardown, podID)
		}
	}

	return setup, teardown
}

func (sm SyncletManager) OnChange(ctx context.Context, store store.RStore) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	setup, teardown := sm.diff(ctx, store)
	for _, podID := range teardown {
		logger.Get(ctx).Debugf("Closing connection to synclet: %s", podID)
		err := sm.forgetPod(ctx, podID)
		if err != nil {
			logger.Get(ctx).Infof("Closing Synclet: %v", err)
		}
	}

	for _, entry := range setup {
		logger.Get(ctx).Debugf("Warming connection to synclet: %s", entry.PodID)
		_, err := sm.clientForPodInternal(ctx, entry.PodID, entry.Namespace)
		if err != nil {
			logger.Get(ctx).Infof("Warming Synclet: %v", err)
		}
	}
}

func (sm SyncletManager) ClientForPod(ctx context.Context, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.clientForPodInternal(ctx, podID, ns)
}

func (sm SyncletManager) clientForPodInternal(ctx context.Context, podID k8s.PodID, ns k8s.Namespace) (synclet.SyncletClient, error) {
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

func (sm SyncletManager) forgetPod(ctx context.Context, podID k8s.PodID) error {
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
	_, err = k8s.WaitForContainerReady(ctx, kCli, pod, container.NewRefSelector(sidecar.SyncletImageRef))
	if err != nil {
		return nil, errors.Wrap(err, "newSyncletClient")
	}

	// TODO(nick): We need a better way to kill the client when the pod dies.
	tunneledPort, tunnelCloser, err := kCli.ForwardPort(ctx, ns, podID, 0, synclet.Port)
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
