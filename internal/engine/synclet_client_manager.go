package engine

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet"
	"google.golang.org/grpc"
)

const syncletAppName = "synclet"
const syncletNamespace = "kube-system"
const syncletOwnerEnvVar = "TILT_SYNCLET_OWNER"

type SyncletClientManager struct {
	kCli    k8s.Client
	mutex   *sync.Mutex
	clients map[k8s.NodeID]synclet.SyncletClient
}

func NewSyncletClientManager(kCli k8s.Client) SyncletClientManager {
	return SyncletClientManager{
		kCli:    kCli,
		mutex:   new(sync.Mutex),
		clients: make(map[k8s.NodeID]synclet.SyncletClient),
	}
}

func (scm SyncletClientManager) ClientForNode(ctx context.Context, nodeID k8s.NodeID) (synclet.SyncletClient, error) {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()

	client, ok := scm.clients[nodeID]
	if ok {
		return client, nil
	}

	client, err := newSyncletClient(ctx, scm.kCli, nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "error creating synclet client")
	}
	scm.clients[nodeID] = client

	return client, nil
}

func newSyncletClient(ctx context.Context, kCli k8s.Client, nodeID k8s.NodeID) (synclet.SyncletClient, error) {
	opts := k8s.FindAppByNodeOptions{Namespace: syncletNamespace}

	syncletOwner, exists := os.LookupEnv(syncletOwnerEnvVar)
	if exists {
		opts.Owner = syncletOwner
	}

	syncletPodID, err := kCli.FindAppByNode(ctx, nodeID, syncletAppName, opts)
	if err != nil {
		if _, ok := err.(k8s.MultipleAppsFoundError); ok {
			return nil, errors.Wrapf(err, "multiple synclet apps found (consider setting $%s to indicate which to use)", syncletOwnerEnvVar)
		}
		return nil, err
	}

	tunneledPort, _, err := kCli.ForwardPort(ctx, syncletNamespace, syncletPodID, synclet.Port)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening tunnel to synclet pod '%s'", syncletPodID)
	}

	logger.Get(ctx).Verbosef("tunneling to synclet client at %s (local port %d)", syncletPodID.String(), tunneledPort)

	t := opentracing.GlobalTracer()

	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", tunneledPort), grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(t)),
		grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(t)))
	if err != nil {
		return nil, errors.Wrap(err, "connecting to synclet")
	}

	return synclet.NewGRPCClient(conn), nil
}
