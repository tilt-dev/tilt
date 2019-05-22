package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/windmilleng/tilt/internal/logger"
)

// wraps a kubectlRunner with logging
type loggingKubectlRunner struct {
	logLevel logger.Level
	runner   kubectlRunner
}

var _ kubectlRunner = loggingKubectlRunner{}

func (k loggingKubectlRunner) logExecStart(ctx context.Context, args []string, stdin io.Reader) (newStdin io.Reader, err error) {
	if k.logLevel == logger.NoneLvl {
		return stdin, nil
	}

	s := fmt.Sprintf("Running: %q\n", append([]string{"kubectl"}, args...))
	logger.Get(ctx).Write(k.logLevel, s)

	if stdin != nil {
		input, err := ioutil.ReadAll(stdin)
		if err != nil {
			return nil, err
		}

		logger.Get(ctx).Write(k.logLevel, fmt.Sprintf("stdin: '%s'\n", string(input)))

		return bytes.NewReader(input), nil
	} else {
		return nil, nil
	}
}

func (k loggingKubectlRunner) logExecStop(ctx context.Context, stdout, stderr string) {
	if k.logLevel == logger.NoneLvl {
		return
	}

	logger.Get(ctx).Write(k.logLevel, fmt.Sprintf("stdout: '%s'\nstderr: '%s'\n", stdout, stderr))
}

func (k loggingKubectlRunner) exec(ctx context.Context, argv []string) (stdout string, stderr string, err error) {
	_, err = k.logExecStart(ctx, argv, nil)
	if err != nil {
		return "", "", err
	}
	stdout, stderr, err = k.runner.exec(ctx, argv)
	k.logExecStop(ctx, stdout, stderr)
	return stdout, stderr, err
}

func (k loggingKubectlRunner) execWithStdin(ctx context.Context, argv []string, stdin io.Reader) (stdout string, stderr string, err error) {
	stdin, err = k.logExecStart(ctx, argv, stdin)
	if err != nil {
		return "", "", err
	}
	stdout, stderr, err = k.runner.execWithStdin(ctx, argv, stdin)
	k.logExecStop(ctx, stdout, stderr)
	return stdout, stderr, err
}

type KubectlLogLevel = logger.Level

func ProvideKubectlRunner(kubeContext KubeContext, logLevel KubectlLogLevel) kubectlRunner {
	return loggingKubectlRunner{
		logLevel: logger.Level(logLevel),
		runner: realKubectlRunner{
			kubeContext: kubeContext,
		},
	}
}
