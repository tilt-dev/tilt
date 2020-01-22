package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type DockerContainerUpdater struct {
	dCli docker.Client
}

var _ ContainerUpdater = &DockerContainerUpdater{}

func NewDockerContainerUpdater(dCli docker.Client) *DockerContainerUpdater {
	return &DockerContainerUpdater{dCli: dCli}
}

func (cu *DockerContainerUpdater) UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	l := logger.Get(ctx)

	err := cu.rmPathsFromContainer(ctx, cInfo.ContainerID, filesToDelete)
	if err != nil {
		return errors.Wrap(err, "rmPathsFromContainer")
	}

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = cu.dCli.CopyToContainerRoot(ctx, cInfo.ContainerID.String(), archiveToCopy)
	if err != nil {
		return err
	}

	// Exec run's on container
	for _, s := range cmds {
		err = cu.dCli.ExecInContainer(ctx, cInfo.ContainerID, s, l.Writer(logger.InfoLvl))
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

func (cu *DockerContainerUpdater) rmPathsFromContainer(ctx context.Context, cID container.ID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	out := bytes.NewBuffer(nil)
	err := cu.dCli.ExecInContainer(ctx, cID, model.Cmd{Argv: makeRmCmd(paths)}, out)
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
