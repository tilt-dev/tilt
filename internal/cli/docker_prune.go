package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/tiltfile"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
)

type dockerPruneCmd struct {
	maxAgeStr string
}

type dpDeps struct {
	dCli docker.Client
	tfl  tiltfile.TiltfileLoader
}

func newDPDeps(dCli docker.Client, tfl tiltfile.TiltfileLoader) dpDeps {
	return dpDeps{
		dCli: dCli,
		tfl:  tfl,
	}
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
	// ❗️ fix this - load Tiltfile and pass image selectors ❗
	dp.Prune(ctx, maxAge, nil)

	return nil
}
