package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// This isn't perfect (because it won't unquote the value right) but
// it's good enough for 99% of cases.
var envMatcher = regexp.MustCompile(`export (\w+)="([^"]+)"`)
var versionMatcher = regexp.MustCompile(`^minikube version: v([0-9.]+)$`)

// Error messages if Minikube is running OK but docker-env is unsupported.
var dockerEnvUnsupportedMsgs = []string{
	"ENV_DRIVER_CONFLICT",
	"ENV_MULTINODE_CONFLICT",
	"ENV_DOCKER_UNAVAILABLE",
	"The docker-env command is only compatible",
}

type MinikubeClient interface {
	Version(ctx context.Context) (string, error)
	DockerEnv(ctx context.Context) (map[string]string, bool, error)
	NodeIP(ctx context.Context) (NodeIP, error)
}

type minikubeClient struct {
	// The minikube client needs to know which minikube profile to talk to.
	//
	// When minikube creates a cluster, it sets the kubeconfig context name and
	// the cluster nameto the name of the profile.
	//
	// The cluster name is better because users
	// don't usually rename it.
	context ClusterName
}

func ProvideMinikubeClient(context ClusterName) MinikubeClient {
	return minikubeClient{context: context}
}

func (mc minikubeClient) cmd(ctx context.Context, args ...string) *exec.Cmd {
	args = append([]string{"-p", string(mc.context)}, args...)
	return exec.CommandContext(ctx, "minikube", args...)
}

func (mc minikubeClient) Version(ctx context.Context) (string, error) {
	cmd := mc.cmd(ctx, "version")
	output, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			return "", fmt.Errorf("Could not read minikube version.\n%s", string(exitErr.Stderr))
		}
		return "", errors.Wrap(err, "Could not read minikube version")
	}
	return minikubeVersionFromOutput(output)

}

func minikubeVersionFromOutput(output []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewBuffer(output))
	for scanner.Scan() {
		line := scanner.Text()

		match := versionMatcher.FindStringSubmatch(line)
		if len(match) > 0 {
			return match[1], nil
		}
	}

	return "", fmt.Errorf("version not found in output:\n%s", string(output))
}

// Returns:
// - A map of env variables for the minikube docker-env.
// - True if this minikube supports a docker-env, false otherwise
// - An error if minikube doesn't appear to be running.
func (mc minikubeClient) DockerEnv(ctx context.Context) (map[string]string, bool, error) {
	cmd := mc.cmd(ctx, "docker-env", "--shell", "sh")
	output, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			stderr := string(exitErr.Stderr)
			for _, msg := range dockerEnvUnsupportedMsgs {
				if strings.Contains(stderr, msg) {
					return nil, false, nil
				}
			}

			return nil, false, fmt.Errorf("Could not read docker env from minikube.\n"+
				"Did you forget to run `minikube start`?\n%s", stderr)
		}
		return nil, false, errors.Wrap(err, "Could not read docker env from minikube")
	}
	return dockerEnvFromOutput(output), true, nil
}

func dockerEnvFromOutput(output []byte) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewBuffer(output))
	for scanner.Scan() {
		line := scanner.Text()

		match := envMatcher.FindStringSubmatch(line)
		if len(match) > 0 {
			result[match[1]] = match[2]
		}
	}

	return result
}

func (mc minikubeClient) NodeIP(ctx context.Context) (NodeIP, error) {
	cmd := mc.cmd(ctx, "ip")
	out, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			return "", errors.Wrapf(exitErr, "Could not read node IP from minikube.\n"+
				"Did you forget to run `minikube start`?\n%s", string(exitErr.Stderr))
		}
		return "", errors.Wrapf(err, "Could not read node IP from minikube")
	}

	return NodeIP(strings.TrimSpace(string(out))), nil
}
