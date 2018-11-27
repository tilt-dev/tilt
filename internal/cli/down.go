package cli

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type downCmd struct {
}

func (c downCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "delete the kubernetes resources associated with manifest(s) defined in Tiltfile.main()",
	}

	return cmd
}

func (c downCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.down", map[string]string{})
	defer analyticsService.Flush(time.Second)

	tf, err := tiltfile.Load(ctx, tiltfile.FileName)
	if err != nil {
		return err
	}

	manifests, gYAML, _, err := tf.GetManifestConfigsAndGlobalYAML(ctx)
	if err != nil {
		return err
	}

	entities, err := engine.ParseYAMLFromManifests(manifests...)
	if err != nil {
		return errors.Wrap(err, "Parsing manifest YAML")
	}

	globalEntities, err := k8s.ParseYAMLFromString(gYAML.K8sYAML())
	if err != nil {
		return errors.Wrap(err, "Parsing global YAML")
	}
	entities = append(entities, globalEntities...)

	kClient, err := wireK8sClient(ctx)
	if err != nil {
		return err
	}

	return kClient.Delete(ctx, entities)
}
