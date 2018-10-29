package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type downCmd struct {
}

func (c downCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down <name> [<name2>] [<name3>] [...]",
		Short: "delete the kubernetes resources in one or more manifests",
		Args:  cobra.MinimumNArgs(1),
	}

	return cmd
}

func (c downCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.down", map[string]string{
		"count": fmt.Sprintf("%d", len(args)),
	})
	defer analyticsService.Flush(time.Second)

	tf, err := tiltfile.Load(ctx, tiltfile.FileName)
	if err != nil {
		return err
	}

	manifests, _, err := tf.GetManifestConfigsAndGlobalYAML(args...)
	if err != nil {
		return err
	}

	// TODO(maia): k8s.delete entities from global yaml as well
	entities, err := engine.ParseYAMLFromManifests(manifests...)
	if err != nil {
		return errors.Wrap(err, "Parsing YAML")
	}

	kClient, err := wireK8sClient(ctx)
	if err != nil {
		return err
	}

	return kClient.Delete(ctx, entities)
}
