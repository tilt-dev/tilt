package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

type kubectlRunner interface {
	exec(ctx context.Context, kubeContext KubeContext, argv []string) (stdout string, stderr string, err error)
	execWithStdin(ctx context.Context, kubeContext KubeContext, argv []string, stdin io.Reader) (stdout string, stderr string, err error)
}

type realKubectlRunner struct{}

var _ kubectlRunner = realKubectlRunner{}

func prependKubeContext(kubeContext KubeContext, args []string) []string {
	return append([]string{fmt.Sprintf("--context=%s", kubeContext)}, args...)
}

func (k realKubectlRunner) exec(ctx context.Context, kubeContext KubeContext, args []string) (stdout string, stderr string, err error) {
	args = prependKubeContext(kubeContext, args)
	c := exec.CommandContext(ctx, "kubectl", args...)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err = c.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func (k realKubectlRunner) execWithStdin(ctx context.Context, kubeContext KubeContext, args []string, stdin io.Reader) (stdout string, stderr string, err error) {
	args = prependKubeContext(kubeContext, args)
	c := exec.CommandContext(ctx, "kubectl", args...)
	c.Stdin = stdin

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err = c.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}
