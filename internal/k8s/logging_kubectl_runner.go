package k8s

import (
	"context"
	"fmt"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// wraps a kubectlRunner with logging
type loggingKubectlRunner struct {
	kubectlLogLevel KubectlLogLevel
	runner          kubectlRunner
}

var _ kubectlRunner = loggingKubectlRunner{}

func (k loggingKubectlRunner) logExecStart(ctx context.Context, args []string, stdin string) {
	if k.kubectlLogLevel == 0 {
		return
	}

	logger.Get(ctx).Infof("Running: %q\n", append([]string{"tilt", "kubectl"}, args...))

	if stdin != "" {
		logger.Get(ctx).Infof("stdin: '%s'\n", stdin)
	}
}

func (k loggingKubectlRunner) logExecStop(ctx context.Context, stdout, stderr string) {
	if k.kubectlLogLevel == 0 {
		return
	}

	logger.Get(ctx).Infof("kubectl stdout: '%s'\nkubectl stderr: '%s'\n", stdout, stderr)
}

func (k loggingKubectlRunner) adjustedVerbosity(argv []string) []string {
	if k.kubectlLogLevel == 0 {
		// don't add -v0 so that in the normal case we're not doing anything surprising
		return argv
	}
	// We're not gonna worry about the case where the input args already have a -v.
	// Empirically, kubectl's behavior is to use the last -v arg it receives, so some call that specifies its own
	//   -v would get its -v, and not whatever the user specified on the command line.
	// This isn't necessarily ideal (maybe we want to use the max, or always prefer k.kubectlLogLevel?), but:
	// 1. We aren't calling this with any other -v at the moment.
	// 2. Doing it right would mean handling ["-v" "4"], ["-v4"], ["-v=4"], and maybe others? really we'd ought to just
	//    call whatever arg parser kubectl uses.
	return append([]string{"-v", fmt.Sprintf("%d", k.kubectlLogLevel)}, argv...)
}

func (k loggingKubectlRunner) exec(ctx context.Context, argv []string) (stdout string, stderr string, err error) {
	argv = k.adjustedVerbosity(argv)
	k.logExecStart(ctx, argv, "")
	stdout, stderr, err = k.runner.exec(ctx, argv)
	k.logExecStop(ctx, stdout, stderr)
	return stdout, stderr, err
}

func (k loggingKubectlRunner) execWithStdin(ctx context.Context, argv []string, stdin string) (stdout string, stderr string, err error) {
	argv = k.adjustedVerbosity(argv)
	k.logExecStart(ctx, argv, stdin)
	stdout, stderr, err = k.runner.execWithStdin(ctx, argv, stdin)
	k.logExecStop(ctx, stdout, stderr)
	return stdout, stderr, err
}

type KubectlLogLevel = int

func ProvideKubectlRunner(kubeContext KubeContext, logLevel KubectlLogLevel) kubectlRunner {
	return loggingKubectlRunner{
		kubectlLogLevel: logLevel,
		runner: realKubectlRunner{
			kubeContext: kubeContext,
		},
	}
}
