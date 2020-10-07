package k8s

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

type kubectlRunner interface {
	exec(ctx context.Context, argv []string) (stdout string, stderr string, err error)
	execWithStdin(ctx context.Context, argv []string, stdin string) (stdout string, stderr string, err error)
}

type realKubectlRunner struct {
	kubeContext KubeContext
}

var _ kubectlRunner = realKubectlRunner{}

func (k realKubectlRunner) tiltPath() (string, error) {
	// TODO(nick): It might be better to dependency inject this. Right now, this
	// only works if the executable is the Tilt binary.  It won't work right if this
	// is linked into separately compiled binaries that don't have a kubectl
	// sub-command.
	path, err := os.Executable()
	return path, errors.Wrap(err, "finding path of Tilt executable (for `tilt kubectl` call)")
}

func (k realKubectlRunner) prependGlobalArgs(args []string) []string {
	return append([]string{"kubectl", "--context", string(k.kubeContext)}, args...)
}

func (k realKubectlRunner) exec(ctx context.Context, args []string) (stdout string, stderr string, err error) {
	args = k.prependGlobalArgs(args)
	tiltPath, err := k.tiltPath()
	if err != nil {
		return "", "", err
	}
	c := exec.CommandContext(ctx, tiltPath, args...)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err = c.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func (k realKubectlRunner) execWithStdin(ctx context.Context, args []string, stdin string) (stdout string, stderr string, err error) {
	args = k.prependGlobalArgs(args)
	tiltPath, err := k.tiltPath()
	if err != nil {
		return "", "", err
	}
	c := exec.CommandContext(ctx, tiltPath, args...)
	c.Stdin = strings.NewReader(stdin)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err = c.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}
