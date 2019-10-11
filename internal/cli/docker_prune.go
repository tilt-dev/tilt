package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
	"github.com/windmilleng/tilt/pkg/logger"
)

type dockerPruneCmd struct {
	untilStr string
}

func (c *dockerPruneCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-prune",
		Short: "run docker prune as Tilt does",
	}

	cmd.Flags().StringVar(&c.untilStr, "until", "6h", "max age of image to keep (as go duration string, e.g. 1h30m, 12h")

	return cmd
}

func (c *dockerPruneCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.dockerPrune", nil)
	a.IncrIfUnopted("analytics.up.optdefault")
	defer a.Flush(time.Second)

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	dCli, err := wireDockerClusterClient(ctx)
	if err != nil {
		return err
	}

	dp := dockerprune.NewDockerPruner(dCli)

	until, err := time.ParseDuration(c.untilStr)
	if err != nil {
		return err
	}

	// TODO: print the commands being run
	dp.Prune(ctx, until)

	return nil
}
