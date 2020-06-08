package cli

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/engine"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type downCmd struct {
	fileName         string
	deleteNamespaces bool
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

Namespaces are not deleted by default. Use --delete-namespaces to change that.

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

	addTiltfileFlag(cmd, &c.fileName)
	cmd.Flags().BoolVar(&c.deleteNamespaces, "delete-namespaces", false, "delete namespaces defined in the Tiltfile (by default, don't)")

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

	if !c.deleteNamespaces {
		var namespaces []k8s.K8sEntity
		entities, namespaces, err = k8s.Filter(entities, func(e k8s.K8sEntity) (b bool, err error) {
			return e.GVK() != schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, nil
		})
		if err != nil {
			return errors.Wrap(err, "filtering out namespaces")
		}
		if len(namespaces) > 0 {
			var nsNames []string
			for _, ns := range namespaces {
				nsNames = append(nsNames, ns.Name())
			}
			logger.Get(ctx).Infof("Not deleting namespaces: %s", strings.Join(nsNames, ", "))
			logger.Get(ctx).Infof("Run with --delete-namespaces to delete namespaces as well.")
		}
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
