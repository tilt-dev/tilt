package k8s

import (
	"context"
	"sync"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// Some K8s environments expose a single IP for the whole cluster.
type NodeIP string

type nodeIPAsync struct {
	mkClient MinikubeClient
	env      Env
	once     sync.Once
	nodeIP   NodeIP
}

func newNodeIPAsync(env Env, mkClient MinikubeClient) *nodeIPAsync {
	return &nodeIPAsync{
		env:      env,
		mkClient: mkClient,
	}
}

func (a *nodeIPAsync) detectNodeIP(ctx context.Context) NodeIP {
	if a.env != EnvMinikube {
		return ""
	}
	nodeIP, err := a.mkClient.NodeIP(ctx)
	if err != nil {
		logger.Get(ctx).Warnf("%s", err.Error())
	}
	return nodeIP
}

func (a *nodeIPAsync) NodeIP(ctx context.Context) NodeIP {
	a.once.Do(func() {
		a.nodeIP = a.detectNodeIP(ctx)
	})
	return a.nodeIP
}

func (c K8sClient) NodeIP(ctx context.Context) NodeIP {
	return c.nodeIPAsync.NodeIP(ctx)
}
