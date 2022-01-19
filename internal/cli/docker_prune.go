package cli

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	apitiltfile "github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/engine/dockerprune"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile"
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

	tlr := deps.tfl.Load(ctx, ctrltiltfile.MainTiltfile(c.fileName, args))
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

func resolveImageSelectors(ctx context.Context, kCli k8s.Client, tlr *tiltfile.TiltfileLoadResult) ([]container.RefSelector, error) {
	registry := apitiltfile.DecideRegistry(ctx, kCli, tlr)
	for _, m := range tlr.Manifests {
		if err := m.InferImagePropertiesFromCluster(registry); err != nil {
			return nil, err
		}
	}

	imgSelectors := model.LocalRefSelectorsForManifests(tlr.Manifests)
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
