package cli

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/tilt-dev/tilt/internal/analytics"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type downCmd struct {
	fileName         string
	deleteNamespaces bool
	downDepsProvider func(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (DownDeps, error)
}

func newDownCmd() *downCmd {
	return &downCmd{downDepsProvider: wireDownDeps}
}

func (c *downCmd) name() model.TiltSubcommand { return "down" }

func (c *downCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "down [<tilt flags>] [-- <Tiltfile args>]",
		DisableFlagsInUseLine: true,
		Short:                 "Delete resources created by 'tilt up'",
		Long: `
Deletes resources specified in the Tiltfile

Specify additional flags and arguments to control which resources are deleted.

Namespaces are not deleted by default. Use --delete-namespaces to change that.

Kubernetes resources with the annotation 'tilt.dev/down-policy: keep' are not deleted.

For more complex cases, the Tiltfile has APIs to add additional flags and arguments to the Tilt CLI.
These arguments can be scripted to define custom subsets of resources to delete.
See https://docs.tilt.dev/tiltfile_config.html for examples.
`,
	}

	addTiltfileFlag(cmd, &c.fileName)
	addKubeContextFlag(cmd)
	cmd.Flags().BoolVar(&c.deleteNamespaces, "delete-namespaces", false, "delete namespaces defined in the Tiltfile (by default, don't)")

	return cmd
}

func (c *downCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.down", map[string]string{})
	defer a.Flush(time.Second)

	downDeps, err := c.downDepsProvider(ctx, a, "down")
	if err != nil {
		return err
	}
	return c.down(ctx, downDeps, args)
}

func (c *downCmd) down(ctx context.Context, downDeps DownDeps, args []string) error {
	tlr := downDeps.tfl.Load(ctx, ctrltiltfile.MainTiltfile(c.fileName, args))
	err := tlr.Error
	if err != nil {
		return err
	}

	if err := deleteK8sEntities(ctx, tlr.Manifests, tlr.UpdateSettings, downDeps, c.deleteNamespaces); err != nil {
		return err
	}

	var dcServices []model.DockerComposeUpSpec
	for _, m := range tlr.Manifests {
		if m.IsDC() {
			dcServices = append(dcServices, m.DockerComposeTarget().Spec)
		}
	}

	if len(dcServices) > 0 {
		dcc := downDeps.dcClient
		// at the time of writing, a Tiltfile can only have one DC project
		proj := dcServices[0].Project

		// `docker-compose down` removes all services in the config
		// if we are only stopping a subset, we use `docker-compose rm` instead
		// (don't *always* use `rm` because it doesn't remove networks)
		useDown, err := isAllServicesInProject(ctx, dcc, proj, dcServices)
		if err != nil {
			return err
		}
		if useDown {
			err := dcc.Down(ctx, proj, logger.Get(ctx).Writer(logger.InfoLvl), logger.Get(ctx).Writer(logger.InfoLvl))
			if err != nil {
				return errors.Wrap(err, "Running `docker-compose down`")
			}
		} else {
			err := dcc.Rm(ctx, dcServices, logger.Get(ctx).Writer(logger.InfoLvl), logger.Get(ctx).Writer(logger.InfoLvl))
			if err != nil {
				return errors.Wrap(err, "Running `docker-compose rm`")
			}
		}
	}

	return nil
}

// returns true iff `services` is the full list of services in `proj`
func isAllServicesInProject(ctx context.Context, dcc dockercompose.DockerComposeClient, proj model.DockerComposeProject, services []model.DockerComposeUpSpec) (bool, error) {
	p, err := dcc.Project(ctx, proj)
	if err != nil {
		return false, errors.Wrap(err, "parsing docker compose project")
	}
	var specifiedServiceNames []string
	for _, s := range services {
		specifiedServiceNames = append(specifiedServiceNames, s.Service)
	}
	serviceNamesInProject := p.ServiceNames()
	return sliceutils.StringSliceSameElements(specifiedServiceNames, serviceNamesInProject), nil
}

func deleteK8sEntities(ctx context.Context, manifests []model.Manifest, updateSettings model.UpdateSettings, downDeps DownDeps, deleteNamespaces bool) error {
	entities, deleteCmds, err := k8sToDelete(manifests...)
	if err != nil {
		return errors.Wrap(err, "Parsing manifest YAML")
	}

	entities = k8s.ReverseSortedEntities(entities)

	entities, _, err = k8s.Filter(entities, func(e k8s.K8sEntity) (b bool, err error) {
		downPolicy, exists := e.Annotations()["tilt.dev/down-policy"]
		return !exists || downPolicy != "keep", nil
	})
	if err != nil {
		return errors.Wrap(err, "Filtering entities by down policy")
	}

	if !deleteNamespaces {
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

	errs := []error{}
	if len(entities) > 0 {
		dCtx, cancel := context.WithTimeout(ctx, updateSettings.K8sUpsertTimeout())
		err = downDeps.kClient.Delete(dCtx, entities)
		cancel()
		if err != nil {
			errs = append(errs, errors.Wrap(err, "Deleting k8s entities"))
		}
	}

	for i := range deleteCmds {
		dCtx, cancel := context.WithTimeout(ctx, updateSettings.K8sUpsertTimeout())
		err := localexec.OneShotToLogger(dCtx, downDeps.execer, deleteCmds[i])
		cancel()

		if err != nil {
			errs = append(errs, errors.Wrapf(err, "Deleting k8s entities for cmd: %s", deleteCmds[i].String()))
		}
	}

	return utilerrors.NewAggregate(errs)
}

func k8sToDelete(manifests ...model.Manifest) ([]k8s.K8sEntity, []model.Cmd, error) {
	var allEntities []k8s.K8sEntity
	var deleteCmds []model.Cmd
	for _, m := range manifests {
		if !m.IsK8s() {
			continue
		}
		kt := m.K8sTarget()

		if kt.DeleteCmd != nil {
			deleteCmds = append(deleteCmds, model.Cmd{
				Argv: kt.DeleteCmd.Args,
				Dir:  kt.DeleteCmd.Dir,
				Env:  kt.DeleteCmd.Env,
			})
		} else {
			entities, err := k8s.ParseYAMLFromString(kt.YAML)
			if err != nil {
				return nil, nil, err
			}
			allEntities = append(allEntities, entities...)
		}
	}
	return allEntities, deleteCmds, nil
}
