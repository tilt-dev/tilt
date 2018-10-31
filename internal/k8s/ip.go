package k8s

import (
	"context"
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

	cmd := exec.CommandContext(ctx, "minikube", "ip")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "DetectNodeIP")
	}

	return NodeIP(strings.TrimSpace(string(out))), nil
}
