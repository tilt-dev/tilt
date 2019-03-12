package minikube

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/pkg/errors"
)

// This isn't perfect (because it won't unquote the value right) but
// it's good enough for 99% of cases.
var envMatcher = regexp.MustCompile(`export (\w+)="([^"]+)"`)

type Client interface {
	DockerEnv(ctx context.Context) (map[string]string, error)
}

func ProvideMinikubeClient() Client {
	return client{}
}

type client struct{}

func (client) DockerEnv(ctx context.Context) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "minikube", "docker-env", "--shell", "sh")
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
