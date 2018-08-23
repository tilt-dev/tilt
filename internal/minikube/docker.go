package minikube

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
)

// This isn't perfect (because it won't unquote the value right) but
// it's good enough for 99% of cases.
var envMatcher = regexp.MustCompile(`export (\w+)="([^"]+)"`)

func DockerEnv(ctx context.Context) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "minikube", "docker-env", "--shell", "sh")
	output, err := cmd.Output()
	if err != nil {
		exitErr, isExitErr := err.(*exec.ExitError)
		if isExitErr {
			return nil, fmt.Errorf("Could not read docker env from minikube.\n%s", string(exitErr.Stderr))
		}

		return nil, fmt.Errorf("Could not read docker env from minikube: %v", err)
	}
	return dockerEnvFromOutput(output)
}

func dockerEnvFromOutput(output []byte) (map[string]string, error) {
	result := make(map[string]string, 0)
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
