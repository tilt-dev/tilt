package minikube

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/k8s"
)

// This isn't perfect (because it won't unquote the value right) but
// it's good enough for 99% of cases.
var envMatcher = regexp.MustCompile(`export (\w+)="([^"]+)"`)

type Client interface {
	DockerEnv(ctx context.Context) (map[string]string, error)
	// NodeIP(ctx context.Context) NodeIP
}

type client struct {
	context k8s.KubeContext
}

func ProvideMinikubeClient(context k8s.KubeContext) Client {
	return client{context: context}
}

func (c client) cmd(ctx context.Context, args ...string) *exec.Cmd {
	args = append([]string{"-p", string(c.context)}, args...)
	return exec.CommandContext(ctx, "minikube", args...)
}

func (c client) DockerEnv(ctx context.Context) (map[string]string, error) {
	cmd := c.cmd(ctx, "docker-env", "--shell", "sh")
	output, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			// TODO(nick): Maybe we should automatically run minikube start?
			return nil, fmt.Errorf("Could not read docker env from minikube.\n"+
				"Did you forget to run `minikube start`?\n%s", string(exitErr.Stderr))
		}

		return nil, errors.Wrap(err, "Could not read docker env from minikube")
	}
	return dockerEnvFromOutput(output)
}

func dockerEnvFromOutput(output []byte) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewBuffer(output))
	for scanner.Scan() {
		line := scanner.Text()

		match := envMatcher.FindStringSubmatch(line)
		if len(match) > 0 {
			result[match[1]] = match[2]
		}
	}

	return result, nil
}

// func (c client) NodeIP(ctx context.Context) NodeIP {
// // TODO(nick): Should this be part of MinikubeClient?
// cmd := exec.CommandContext(ctx, "minikube", "ip")
// out, err := cmd.Output()
// if err != nil {
// exitErr, isExitErr := err.(*exec.ExitError)
// if isExitErr {
// // TODO(nick): Maybe we should automatically run minikube start?
// logger.Get(ctx).Warnf("Could not read node IP from minikube.\n"+
// "Did you forget to run `minikube start`?\n%s", string(exitErr.Stderr))
// } else {
// logger.Get(ctx).Warnf("Could not read node IP from minikube")
// }
// return ""
// }
//
// return NodeIP(strings.TrimSpace(string(out)))
