package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/tiltfile"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
)

type dockerPruneCmd struct {
	maxAgeStr string
	fileName  string
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
	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")

	return cmd
}

func (c *dockerPruneCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.dockerPrune", nil)
	a.IncrIfUnopted("analytics.up.optdefault")
	defer a.Flush(time.Second)

	deps, err := wireDockerPrune(ctx, a)
	if err != nil {
		return err
	}

	tlr := deps.tfl.Load(ctx, c.fileName, nil)
	if tlr.Error != nil {
		return tlr.Error
	}

	imgSelectors := model.RefSelectorsForManifests(tlr.Manifests)

	dp := dockerprune.NewDockerPruner(deps.dCli)

	maxAge, err := time.ParseDuration(c.maxAgeStr)
	if err != nil {
		return err
	}

	// TODO: print the commands being run
	dp.Prune(ctx, maxAge, imgSelectors)

	return nil
}
