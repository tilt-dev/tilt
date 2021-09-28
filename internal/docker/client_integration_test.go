//+build !skipcontainertests

package docker

import (
	"bytes"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/require"

	wmcontainer "github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/testutils"
)

func TestCli_Run(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	dEnv := ProvideClusterEnv(ctx, "gke", k8s.EnvGKE, wmcontainer.RuntimeDocker, k8s.FakeMinikube{})
	cli := NewDockerClient(ctx, Env(dEnv))
	defer func() {
		// release any idle connections to avoid out of file errors if running test many times
		_ = cli.(*Cli).Close()
	}()

	ref, err := reference.ParseNamed("docker.io/library/hello-world")
	require.NoError(t, err)

	var stdout bytes.Buffer
	r, err := cli.Run(ctx, RunConfig{
		Pull:   true,
		Image:  ref,
		Stdout: &stdout,
	})
	require.NoError(t, err, "Error during run")
	exitCode, err := r.Wait()
	require.NoError(t, err, "Error waiting for exit")
	require.NoError(t, r.Close(), "Error cleaning up container")
	require.Equal(t, int64(0), exitCode, "Non-zero exit code from container")
	require.Contains(t, stdout.String(), "Hello from Docker", "Bad stdout")
}
