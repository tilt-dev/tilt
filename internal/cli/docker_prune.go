package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
)

type dockerPruneCmd struct {
	maxAgeStr string
}

func (c *dockerPruneCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-prune",
		Short: "run docker prune as Tilt does",
	}

	cmd.Flags().StringVar(&c.maxAgeStr, "maxAge", "6h", "max age of image to keep (as go duration string, e.g. 1h30m, 12h")

	return cmd
}

func (c *dockerPruneCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.dockerPrune", nil)
	a.IncrIfUnopted("analytics.up.optdefault")
	defer a.Flush(time.Second)

	dCli, err := wireDockerClusterClient(ctx)
	if err != nil {
		return err
	}

	dp := dockerprune.NewDockerPruner(dCli)

	maxAge, err := time.ParseDuration(c.maxAgeStr)
	if err != nil {
		return err
	}

	// TODO: print the commands being run
	dp.Prune(ctx, maxAge)

	return nil
}
