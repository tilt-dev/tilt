package k8s

import (
	"context"
	"os/exec"
	"strings"
	"sync"

	"github.com/windmilleng/tilt/pkg/logger"
)

// Some K8s environments expose a single IP for the whole cluster.
type NodeIP string

type nodeIPAsync struct {
	env    Env
	once   sync.Once
	nodeIP NodeIP
}

func newNodeIPAsync(env Env) *nodeIPAsync {
	return &nodeIPAsync{
		env: env,
	}
}

func (a *nodeIPAsync) detectNodeIP(ctx context.Context) NodeIP {
	if a.env != EnvMinikube {
		return ""
	}

	// TODO(nick): Should this be part of MinikubeClient?
	cmd := exec.CommandContext(ctx, "minikube", "ip")
	out, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			// TODO(nick): Maybe we should automatically run minikube start?
			logger.Get(ctx).Warnf("Could not read node IP from minikube.\n"+
				"Did you forget to run `minikube start`?\n%s", string(exitErr.Stderr))
		} else {
			logger.Get(ctx).Warnf("Could not read node IP from minikube")
		}
		return ""
	}

	return NodeIP(strings.TrimSpace(string(out)))
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
