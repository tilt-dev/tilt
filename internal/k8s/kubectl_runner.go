package k8s

import (
	"bytes"
	"context"
	"io"
	"os/exec"

	"github.com/windmilleng/tilt/internal/output"
)

type kubectlRunner interface {
	exec(ctx context.Context, argv []string) (stdout string, stderr string, err error)
	execWithStdin(ctx context.Context, argv []string, stdin io.Reader) (stdout string, stderr string, err error)
}

type realKubectlRunner struct{}

var _ kubectlRunner = realKubectlRunner{}

func (k realKubectlRunner) exec(ctx context.Context, args []string) (stdout string, stderr string, err error) {
	c := exec.CommandContext(ctx, "kubectl", args...)

	writer := output.Get(ctx).Writer()

	stdoutBuf := &bytes.Buffer{}
	c.Stdout = io.MultiWriter(stdoutBuf, writer)

	stderrBuf := &bytes.Buffer{}
	c.Stderr = io.MultiWriter(stderrBuf, writer)

	return stdoutBuf.String(), stderrBuf.String(), c.Run()
}

func (k realKubectlRunner) execWithStdin(ctx context.Context, args []string, stdin io.Reader) (stdout string, stderr string, err error) {
	c := exec.CommandContext(ctx, "kubectl", args...)
	c.Stdin = stdin

	writer := output.Get(ctx).Writer()

	stdoutBuf := &bytes.Buffer{}
	c.Stdout = io.MultiWriter(stdoutBuf, writer)

	stderrBuf := &bytes.Buffer{}
	c.Stderr = io.MultiWriter(stderrBuf, writer)

	return stdoutBuf.String(), stderrBuf.String(), c.Run()
}
