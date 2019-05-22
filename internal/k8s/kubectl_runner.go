package k8s

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

type kubectlRunner interface {
	exec(ctx context.Context, argv []string) (stdout string, stderr string, err error)
	execWithStdin(ctx context.Context, argv []string, stdin io.Reader) (stdout string, stderr string, err error)
}

type realKubectlRunner struct {
	kubeContext KubeContext
}

var _ kubectlRunner = realKubectlRunner{}

func (k realKubectlRunner) prependGlobalArgs(args []string) []string {
	return append([]string{"--context", string(k.kubeContext)}, args...)
}

func (k realKubectlRunner) exec(ctx context.Context, args []string) (stdout string, stderr string, err error) {
	args = k.prependGlobalArgs(args)
	c := exec.CommandContext(ctx, "kubectl", args...)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err = c.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func (k realKubectlRunner) execWithStdin(ctx context.Context, args []string, stdin io.Reader) (stdout string, stderr string, err error) {
	args = k.prependGlobalArgs(args)
	c := exec.CommandContext(ctx, "kubectl", args...)
	c.Stdin = stdin

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err = c.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}
