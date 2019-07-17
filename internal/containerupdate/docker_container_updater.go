package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerContainerUpdater struct {
	dCli docker.Client
}

func NewDockerContainerUpdater(dCli docker.Client) *DockerContainerUpdater {
	return &DockerContainerUpdater{dCli: dCli}
}

func (cu *DockerContainerUpdater) UpdateInContainer(ctx context.Context, deployInfo store.DeployInfo, paths []build.PathMapping, filter model.PathMatcher, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "UpdateInContainer")
	defer span.Finish()
	l := logger.Get(ctx)

	// rm files from container
	toRemove, toArchive, err := build.MissingLocalPaths(ctx, paths)
	if err != nil {
		return errors.Wrap(err, "MissingLocalPaths")
	}

	if len(toRemove) > 0 {
		l.Infof("Will delete %d file(s) from container: %s", len(toRemove), deployInfo.ContainerID.ShortStr())
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	// copy files to container
	pr, pw := io.Pipe()
	go func() {
		ab := build.NewArchiveBuilder(pw, filter)
		err = ab.ArchivePathsIfExist(ctx, toArchive)
		if err != nil {
			_ = pw.CloseWithError(errors.Wrap(err, "archivePathsIfExists"))
		} else {
			_ = ab.Close()
			_ = pw.Close()
		}
	}()

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container: %s", len(toArchive), deployInfo.ContainerID.ShortStr())
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	return cu.UpdateContainer(ctx, deployInfo, pr, build.PathMappingsToContainerPaths(toRemove), cmds, hotReload)
}

func (cu *DockerContainerUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	l := logger.Get(ctx)

	err := cu.RmPathsFromContainer(ctx, deployInfo.ContainerID, filesToDelete)
	if err != nil {
		return errors.Wrap(err, "RmPathsFromContainer")
	}

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = cu.dCli.CopyToContainerRoot(ctx, deployInfo.ContainerID.String(), archiveToCopy)
	if err != nil {
		return err
	}

	// Exec run's on container
	for _, s := range cmds {
		err = cu.dCli.ExecInContainer(ctx, deployInfo.ContainerID, s, l.Writer(logger.InfoLvl))
		if err != nil {
			return build.WrapContainerExecError(err, deployInfo.ContainerID, s)
		}
	}

	if hotReload {
		l.Debugf("Hot reload on, skipping container restart: %s", deployInfo.ContainerID.ShortStr())
		return nil
	}

	// Restart container so that entrypoint restarts with the updated files etc.
	l.Debugf("Restarting container: %s", deployInfo.ContainerID.ShortStr())
	err = cu.dCli.ContainerRestartNoWait(ctx, deployInfo.ContainerID.String())
	if err != nil {
		return errors.Wrap(err, "ContainerRestart")
	}
	return nil
}

func (cu *DockerContainerUpdater) RmPathsFromContainer(ctx context.Context, cID container.ID, paths []string) error {
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
	for _, p := range paths {
		cmd = append(cmd, p)
	}
	return cmd
}
