package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type DockerUpdater struct {
	dCli docker.Client
}

var _ ContainerUpdater = &DockerUpdater{}

func NewDockerUpdater(dCli docker.Client) *DockerUpdater {
	return &DockerUpdater{dCli: dCli}
}

func (cu *DockerUpdater) WillBuildToKubeContext(kctx k8s.KubeContext) bool {
	return cu.dCli.Env().WillBuildToKubeContext(kctx)
}

func (cu *DockerUpdater) UpdateContainer(ctx context.Context, cInfo liveupdates.Container,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	l := logger.Get(ctx)

	err := cu.rmPathsFromContainer(ctx, cInfo.ContainerID, filesToDelete)
	if err != nil {
		return errors.Wrap(err, "rmPathsFromContainer")
	}

	// Use `tar` to unpack the files into the container.
	//
	// Although docker has a copy API, it's buggy and not well-maintained
	// (whereas the Exec API is part of the CRI and much more battle-tested).
	// Discussion:
	// https://github.com/tilt-dev/tilt/issues/3708
	err = cu.dCli.ExecInContainer(ctx, cInfo.ContainerID, model.Cmd{
		Argv: tarArgv(),
	}, archiveToCopy, l.Writer(logger.InfoLvl))
	if err != nil {
		return errors.Wrap(err, "copying files")
	}

	// Exec run's on container
	for _, s := range cmds {
		err = cu.dCli.ExecInContainer(ctx, cInfo.ContainerID, s, nil, l.Writer(logger.InfoLvl))
		if err != nil {
			return build.WrapContainerExecError(err, cInfo.ContainerID, s)
		}
	}

	if hotReload {
		l.Debugf("Hot reload on, skipping container restart: %s", cInfo.ContainerID.ShortStr())
		return nil
	}

	// Restart container so that entrypoint restarts with the updated files etc.
	l.Debugf("Restarting container: %s", cInfo.ContainerID.ShortStr())
	err = cu.dCli.ContainerRestartNoWait(ctx, cInfo.ContainerID.String())
	if err != nil {
		return errors.Wrap(err, "ContainerRestart")
	}
	return nil
}

func (cu *DockerUpdater) rmPathsFromContainer(ctx context.Context, cID container.ID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	out := bytes.NewBuffer(nil)
	err := cu.dCli.ExecInContainer(ctx, cID, model.Cmd{Argv: makeRmCmd(paths)}, nil, out)
	if err != nil {
		if docker.IsExitError(err) {
			return fmt.Errorf("Error deleting files from container: %s", out.String())
		}
		return errors.Wrap(err, "Error deleting files from container")
	}
	return nil
}

func makeRmCmd(paths []string) []string {
	cmd := []string{"rm", "-rf"}
	cmd = append(cmd, paths...)
	return cmd
}
