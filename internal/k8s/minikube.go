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

type MinikubeClient interface {
	Version(ctx context.Context) (string, error)
	DockerEnv(ctx context.Context) (map[string]string, error)
	NodeIP(ctx context.Context) (NodeIP, error)
}

type minikubeClient struct {
	context KubeContext
}

func ProvideMinikubeClient(context KubeContext) MinikubeClient {
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

func (mc minikubeClient) DockerEnv(ctx context.Context) (map[string]string, error) {
	cmd := mc.cmd(ctx, "docker-env", "--shell", "sh")
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
