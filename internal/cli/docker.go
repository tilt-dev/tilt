package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/docker"
)

type dockerCmd struct {
}

func (c *dockerCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docker [flags] -- command ...",
		Short:   "Execute Docker commands as Tilt would execute them",
		Example: "tilt docker -- build -f path/to/Dockerfile .",
	}
	return cmd
}

func (c *dockerCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.docker", map[string]string{})
	defer a.Flush(time.Second)

	client, err := wireDockerClusterClient(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to init Docker client")
	}

	err = client.CheckConnected()
	if err != nil {
		return errors.Wrap(err, "Failed to connect to Docker server")
	}

	dockerEnv := client.Env()
	builder := client.BuilderVersion()

	buildkitEnv := "DOCKER_BUILDKIT=0"
	if builder == types.BuilderBuildKit {
		buildkitEnv = "DOCKER_BUILDKIT=1"
	}
	env := append([]string{buildkitEnv}, docker.Env(dockerEnv).AsEnviron()...)
	fmt.Fprintf(os.Stderr,
		"Running Docker command as:\n%s docker %s\n---\n",
		strings.Join(env, " "),
		strings.Join(args, " "))

	cmd := exec.Command("docker", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
