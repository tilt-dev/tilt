package k8s

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// Some K8s environments expose a single IP for the whole cluster.
type NodeIP string

func DetectNodeIP(ctx context.Context, env Env) (NodeIP, error) {
	if env != EnvMinikube {
		return "", nil
	}

	// TODO(nick): Should this be part of MinikubeClient?
	cmd := exec.CommandContext(ctx, "minikube", "ip")
	out, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			// TODO(nick): Maybe we should automatically run minikube start?
			return "", fmt.Errorf("Could not read node IP from minikube.\n"+
				"Did you forget to run `minikube start`?\n%s", string(exitErr.Stderr))
		}
		return "", errors.Wrap(err, "Could not read node IP from minikube")
	}

	return NodeIP(strings.TrimSpace(string(out))), nil
}
