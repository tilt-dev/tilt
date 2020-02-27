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
	fileName         string
	downDepsProvider func(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics) (DownDeps, error)
}

func newDownCmd() *downCmd {
	return &downCmd{downDepsProvider: wireDownDeps}
}

func (c *downCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "down [<tilt flags>] [-- <Tiltfile args>]",
		DisableFlagsInUseLine: true,
		Short:                 "Delete resources created by 'tilt up'",
		Long: `
Deletes resources specified in the Tiltfile

There are two types of args:
1) Tilt flags, listed below, which are handled entirely by Tilt.
2) Tiltfile args, which can be anything, and are potentially accessed by config.parse in your Tiltfile.

By default:
1) Tiltfile args are interpreted as the list of services to delete, e.g. tilt down frontend backend.
2) Running with no Tiltfile args deletes all services defined in the Tiltfile

This default behavior does not apply if the Tiltfile uses config.parse or config.set_enabled_resources.
In that case, see https://tilt.dev/user_config.html and/or comments in your Tiltfile
`,
	}

	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")

	return cmd
}

func (c *downCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.down", map[string]string{})
	defer a.Flush(time.Second)

	downDeps, err := c.downDepsProvider(ctx, a)
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
