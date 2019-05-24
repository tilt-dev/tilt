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
	"github.com/windmilleng/tilt/internal/analytics"
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

	dockerEnv, err := wireDockerEnv(ctx)
	if err != nil {
		return errors.Wrap(err, "wireDockerEnv")
	}

	builder, err := wireDockerBuilderVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "wireDockerBuilderVersion")
	}

	buildkitEnv := "DOCKER_BUILDKIT=0"
	if builder == types.BuilderBuildKit {
		buildkitEnv = "DOCKER_BUILDKIT=1"
	}
	env := append([]string{buildkitEnv}, dockerEnv.AsEnviron()...)
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
