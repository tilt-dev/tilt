package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/windmilleng/tilt/internal/output"
)

type kubectlRunner interface {
	cli(ctx context.Context, cmd string, entities ...K8sEntity) (stdout string, stderr string, err error)
}

type realKubectlRunner struct{}

var _ kubectlRunner = realKubectlRunner{}

func (k realKubectlRunner) cli(ctx context.Context, cmd string, entities ...K8sEntity) (stdout string, stderr string, err error) {
	args := []string{cmd}

	if len(entities) > 0 {
		args = append(args, "-f", "-")
	}

	c := exec.CommandContext(ctx, "kubectl", args...)

	if len(entities) > 0 {
		rawYAML, err := SerializeYAML(entities)
		if err != nil {
			return "", "", fmt.Errorf("kubectl %s: %v", cmd, err)
		}
		r := bytes.NewReader([]byte(rawYAML))
		c.Stdin = r
	}

	writer := output.Get(ctx).Writer()

	stdoutBuf := &bytes.Buffer{}
	c.Stdout = io.MultiWriter(stdoutBuf, writer)

	stderrBuf := &bytes.Buffer{}
	c.Stderr = io.MultiWriter(stderrBuf, writer)

	return stdoutBuf.String(), stderrBuf.String(), c.Run()
}
