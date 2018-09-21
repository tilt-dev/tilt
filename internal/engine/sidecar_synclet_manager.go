package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet"
	"google.golang.org/grpc"
)

type SidecarSyncletManager struct {
	kCli    k8s.Client
	mutex   *sync.Mutex
	clients map[k8s.PodID]synclet.SyncletClient
}

func NewSidecarSyncletManager(kCli k8s.Client) SidecarSyncletManager {
	return SidecarSyncletManager{
		kCli:    kCli,
		mutex:   new(sync.Mutex),
		clients: make(map[k8s.PodID]synclet.SyncletClient),
	}
}

func (ssm SidecarSyncletManager) ClientForPod(ctx context.Context, podID k8s.PodID) (synclet.SyncletClient, error) {
	ssm.mutex.Lock()
	defer ssm.mutex.Unlock()

	client, ok := ssm.clients[podID]
	if ok {
		return client, nil
	}

	client, err := newSidecarSyncletClient(ctx, ssm.kCli, podID)
	if err != nil {
		return nil, errors.Wrap(err, "error creating synclet client")
	}
	ssm.clients[podID] = client

	return client, nil
}

func newSidecarSyncletClient(ctx context.Context, kCli k8s.Client, podID k8s.PodID) (synclet.SyncletClient, error) {
	// TODO(nick): We need a better way to kill the client when the pod dies.
	tunneledPort, _, err := kCli.ForwardPort(ctx, "default", podID, synclet.Port)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening tunnel to synclet pod '%s'", podID)
	}

	logger.Get(ctx).Verbosef("tunneling to synclet client at %s (local port %d)", podID.String(), tunneledPort)

	t := opentracing.GlobalTracer()

	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", tunneledPort), grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(t)),
		grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(t)))
	if err != nil {
		return nil, errors.Wrap(err, "connecting to synclet")
	}

	return synclet.NewGRPCClient(conn), nil
}
