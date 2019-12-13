package cli

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type downCmd struct {
	fileName string
}

func (c *downCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "delete resources created by 'tilt up'",
		Args:  cobra.NoArgs,
	}

	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")

	return cmd
}

func (c *downCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.down", map[string]string{})
	defer a.Flush(time.Second)

	downDeps, err := wireDownDeps(ctx, a)
	if err != nil {
		return err
	}
	return c.down(ctx, downDeps, args)
}

func (c *downCmd) down(ctx context.Context, downDeps DownDeps, args []string) error {
	tlr := downDeps.tfl.Load(ctx, c.fileName, model.NewUserConfigState(args))
	err := tlr.Error
	if err != nil {
		return err
	}

	entities, err := engine.ParseYAMLFromManifests(tlr.Manifests...)
	if err != nil {
		return errors.Wrap(err, "Parsing manifest YAML")
	}

	if len(entities) > 0 {
		err = downDeps.kClient.Delete(ctx, entities)
		if err != nil {
			return errors.Wrap(err, "Deleting k8s entities")
		}
	}

	var dcConfigPaths []string
	for _, m := range tlr.Manifests {
		if m.IsDC() {
			dcConfigPaths = m.DockerComposeTarget().ConfigPaths
			break
		}
	}

	if len(dcConfigPaths) > 0 {
		dcc := downDeps.dcClient
		err = dcc.Down(ctx, dcConfigPaths, logger.Get(ctx).Writer(logger.InfoLvl), logger.Get(ctx).Writer(logger.InfoLvl))
		if err != nil {
			return errors.Wrap(err, "Running `docker-compose down`")
		}
	}

	return nil
}
