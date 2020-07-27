package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
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

func (cu *DockerUpdater) UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerUpdater-UpdateContainer")
	defer span.Finish()

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

func (cu *DockerUpdater) rmPathsFromContainer(ctx context.Context, cID container.ID, paths []string) error {
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
