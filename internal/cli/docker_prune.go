package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/engine/dockerprune"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type dockerPruneCmd struct {
	fileName string
}

type dpDeps struct {
	dCli docker.Client
	kCli k8s.Client
	tfl  tiltfile.TiltfileLoader
}

func newDPDeps(dCli docker.Client, kCli k8s.Client, tfl tiltfile.TiltfileLoader) dpDeps {
	return dpDeps{
		dCli: dCli,
		kCli: kCli,
		tfl:  tfl,
	}
}

func (c *dockerPruneCmd) name() model.TiltSubcommand { return "docker-prune" }

func (c *dockerPruneCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-prune",
		Short: "Run docker prune as Tilt does",
	}

	addTiltfileFlag(cmd, &c.fileName)

	return cmd
}

func (c *dockerPruneCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.dockerPrune", nil)
	defer a.Flush(time.Second)

	if !logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		// Docker Pruner filters output when nothing is pruned if not in verbose
		// logging mode, which is suitable for when it runs in the background
		// during `tilt up`, but we always want to include that for the CLI cmd
		// N.B. we only override if we're not already showing verbose so that
		// 	`--debug` flag isn't impacted
		l := logger.NewLogger(logger.VerboseLvl, os.Stdout)
		ctx = logger.WithLogger(ctx, l)
	}

	deps, err := wireDockerPrune(ctx, a, "docker-prune")
	if err != nil {
		return err
	}

	tlr := deps.tfl.Load(ctx, ctrltiltfile.MainTiltfile(c.fileName, args), nil)
	if tlr.Error != nil {
		return tlr.Error
	}

	imgSelectors, err := resolveImageSelectors(ctx, deps.kCli, &tlr)
	if err != nil {
		return err
	}

	dp := dockerprune.NewDockerPruner(deps.dCli)

	// TODO: print the commands being run
	dp.Prune(ctx, tlr.DockerPruneSettings.MaxAge, tlr.DockerPruneSettings.KeepRecent, imgSelectors)

	return nil
}

// resolveImageSelectors finds image references from a tiltfile.TiltfileLoadResult object.
//
// The Kubernetes client is used to resolve the correct image names if a local registry is in use.
//
// This method is brittle and duplicates some logic from the actual reconcilers.
// In the future, we hope to have a mode where we can launch the full apiserver
// with all resources in a "disabled" state and rely on the API, but that's not
// possible currently.
func resolveImageSelectors(ctx context.Context, kCli k8s.Client, tlr *tiltfile.TiltfileLoadResult) ([]container.RefSelector, error) {
	if err := model.InferImageProperties(tlr.Manifests); err != nil {
		return nil, err
	}

	var reg *v1alpha1.RegistryHosting
	if tlr.HasOrchestrator(model.OrchestratorK8s) {
		// k8s.Client::LocalRegistry will return an empty registry on any error,
		// so ensure the client is actually functional first
		if _, err := kCli.CheckConnected(ctx); err != nil {
			return nil, fmt.Errorf("determining local registry: %v", err)
		}
		reg = kCli.LocalRegistry(ctx)
	}

	clusters := map[string]*v1alpha1.Cluster{
		v1alpha1.ClusterNameDefault: {
			ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ClusterNameDefault},
			Spec:       v1alpha1.ClusterSpec{DefaultRegistry: reg},
		},
	}

	imgSelectors := model.LocalRefSelectorsForManifests(tlr.Manifests, clusters)
	if len(imgSelectors) != 0 && logger.Get(ctx).Level().ShouldDisplay(logger.DebugLvl) {
		var sb strings.Builder
		for _, is := range imgSelectors {
			sb.WriteString("  - ")
			sb.WriteString(is.RefFamiliarString())
			sb.WriteRune('\n')
		}

		logger.Get(ctx).Debugf("Running Docker Prune for images:\n%s", sb.String())
	}

	return imgSelectors, nil
}
