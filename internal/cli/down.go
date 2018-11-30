package cli

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
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

	manifestNames := make([]model.ManifestName, len(args))
	for i, a := range args {
		manifestNames[i] = model.ManifestName(a)
	}

	manifests, gYAML, _, err := tf.GetManifestConfigsAndGlobalYAML(ctx, manifestNames...)
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

	err = kClient.Delete(ctx, entities)
	if err != nil {
		logger.Get(ctx).Infof("error deleting k8s entities: %v", err)
	}

	var dcManifests []model.Manifest
	for _, m := range manifests {
		if m.IsDockerCompose() {
			dcManifests = append(dcManifests, m)
		}
	}

	if len(dcManifests) > 0 {
		// TODO(maia): when we support up-ing from multiple docker-compose files, we'll need to support down-ing as well
		// TODO(maia): a way to `down` specific services?
		cmd := exec.CommandContext(ctx, "docker-compose", "-f", dcManifests[0].DcYAMLPath, "down")
		cmd.Stdout = logger.Get(ctx).Writer(logger.InfoLvl)
		cmd.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

		err = cmd.Run()
		err = dockercompose.FormatError(cmd, nil, err)
		if err != nil {
			logger.Get(ctx).Infof("error running `docker-compose down`: %v", err)
		}
	}
	return nil
}
