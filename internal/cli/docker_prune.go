package cli

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/tiltfile"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
)

type dockerPruneCmd struct {
	fileName string
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
		Short: "Run docker prune as Tilt does",
	}

	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")

	return cmd
}

func (c *dockerPruneCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.dockerPrune", nil)
	defer a.Flush(time.Second)

	// (Most relevant output from dockerpruner is at the `debug` level)
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	deps, err := wireDockerPrune(ctx, a)
	if err != nil {
		return err
	}

	tlr := deps.tfl.Load(ctx, c.fileName, model.NewUserConfigState(args))
	if tlr.Error != nil {
		return tlr.Error
	}

	imgSelectors := model.LocalRefSelectorsForManifests(tlr.Manifests)

	dp := dockerprune.NewDockerPruner(deps.dCli)

	// TODO: print the commands being run
	dp.Prune(ctx, tlr.DockerPruneSettings.MaxAge, imgSelectors)

	return nil
}
