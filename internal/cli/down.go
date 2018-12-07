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
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tiltfile2"
)

type downCmd struct {
}

func (c downCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "delete kubernetes resources",
	}

	return cmd
}

func (c downCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.down", map[string]string{
		"count": fmt.Sprintf("%d", len(args)),
	})
	defer analyticsService.Flush(time.Second)

	manifests, _, _, err := tiltfile2.Load(ctx, tiltfile.FileName, nil)
	if err != nil {
		return err
	}

	entities, err := engine.ParseYAMLFromManifests(manifests...)
	if err != nil {
		return errors.Wrap(err, "Parsing manifest YAML")
	}

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
