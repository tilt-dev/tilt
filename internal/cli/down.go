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
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type downCmd struct {
	fileName         string
	deleteNamespaces bool
	downDepsProvider func(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (DownDeps, error)
}

type dependencyNode struct {
	manifest   model.Manifest
	dependents []*dependencyNode
	processed  bool
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
	tlr := downDeps.tfl.Load(ctx, ctrltiltfile.MainTiltfile(c.fileName, args), nil)
	err := tlr.Error
	if err != nil {
		return err
	}

	sortedManifests := sortManifestsForDeletion(tlr.Manifests, tlr.EnabledManifests)

	if err := deleteK8sEntities(ctx, sortedManifests, tlr.UpdateSettings, downDeps, c.deleteNamespaces); err != nil {
		return err
	}

	dcProjects := make(map[string]v1alpha1.DockerComposeProject)
	for _, m := range sortedManifests {
		if !m.IsDC() {
			continue
		}
		proj := m.DockerComposeTarget().Spec.Project

		if _, exists := dcProjects[proj.Name]; !exists {
			dcProjects[proj.Name] = proj
		}
	}

	for _, dcProject := range dcProjects {
		if !model.IsEmptyDockerComposeProject(dcProject) {
			dcc := downDeps.dcClient
			err = dcc.Down(ctx, dcProject, logger.Get(ctx).Writer(logger.InfoLvl), logger.Get(ctx).Writer(logger.InfoLvl))
			if err != nil {
				return errors.Wrap(err, "Running `docker-compose down`")
			}
		}
	}

	return nil
}

func sortManifestsForDeletion(manifests []model.Manifest, enabledManifests []model.ManifestName) []model.Manifest {
	enabledNames := make(map[model.ManifestName]bool, len(enabledManifests))
	for _, n := range enabledManifests {
		enabledNames[n] = true
	}

	nodes := []*dependencyNode{}
	nodeMap := map[model.ManifestName]*dependencyNode{}

	for i := range manifests {
		manifest := manifests[len(manifests)-i-1]

		node := &dependencyNode{
			manifest:   manifest,
			dependents: []*dependencyNode{},
		}

		nodes = append(nodes, node)
		nodeMap[manifest.Name] = node
	}

	for _, node := range nodes {
		for _, resourceDep := range node.manifest.ResourceDependencies {
			if dependency, ok := nodeMap[resourceDep]; ok {
				dependency.dependents = append(dependency.dependents, node)
			}
		}
	}

	// The tiltfile loader returns all manifests,
	// with the ones that weren't selected disabled.
	var sortedManifests []model.Manifest
	for _, node := range nodes {
		for _, m := range manifestsForNode(node) {
			if enabledNames[m.Name] {
				sortedManifests = append(sortedManifests, m)
			}
		}
	}

	return sortedManifests
}

func manifestsForNode(node *dependencyNode) []model.Manifest {
	if node.processed {
		return []model.Manifest{}
	}

	node.processed = true

	var manifests []model.Manifest

	for _, dependent := range node.dependents {
		manifests = append(manifests, manifestsForNode(dependent)...)
	}

	return append(manifests, node.manifest)
}

func deleteK8sEntities(ctx context.Context, manifests []model.Manifest, updateSettings model.UpdateSettings, downDeps DownDeps, deleteNamespaces bool) error {
	entities, deleteCmds, err := k8sToDelete(manifests...)
	if err != nil {
		return errors.Wrap(err, "Parsing manifest YAML")
	}

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
		err = downDeps.kClient.Delete(dCtx, entities, false)
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
			allEntities = append(allEntities, k8s.ReverseSortedEntities(entities)...)
		}
	}
	return allEntities, deleteCmds, nil
}
