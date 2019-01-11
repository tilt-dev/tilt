package build

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type ContainerUpdater struct {
	dCli docker.Client
}

func NewContainerUpdater(dCli docker.Client) *ContainerUpdater {
	return &ContainerUpdater{dCli: dCli}
}

func (r *ContainerUpdater) UpdateInContainer(ctx context.Context, cID container.ID, paths []PathMapping, filter model.PathMatcher, steps []model.Cmd, w io.Writer) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-UpdateInContainer")
	defer span.Finish()

	// rm files from container
	toRemove, err := MissingLocalPaths(ctx, paths)
	if err != nil {
		return errors.Wrap(err, "MissingLocalPaths")
	}

	err = r.RmPathsFromContainer(ctx, cID, toRemove)
	if err != nil {
		return errors.Wrap(err, "RmPathsFromContainer")
	}

	// copy files to container
	ab := NewArchiveBuilder(filter)
	err = ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return errors.Wrap(err, "archivePathsIfExists")
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return err
	}

	logger.Get(ctx).Debugf("Copying files to container: %s", cID.ShortStr())

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = r.dCli.CopyToContainerRoot(ctx, cID.String(), bytes.NewReader(archive.Bytes()))
	if err != nil {
		return err
	}

	// Exec steps on container
	for _, s := range steps {
		err = r.dCli.ExecInContainer(ctx, cID, s, w)
		if err != nil {
			exitErr, isExitErr := err.(docker.ExitError)
			if isExitErr {
				return UserBuildFailure{ExitCode: exitErr.ExitCode}
			}
			return errors.Wrapf(err, "executing step %v on container %s", s.Argv, cID.ShortStr())
		}
	}

	// Restart container so that entrypoint restarts with the updated files etc.
	err = r.dCli.ContainerRestartNoWait(ctx, cID.String())
	if err != nil {
		return errors.Wrap(err, "ContainerRestart")
	}
	return nil
}

func (r *ContainerUpdater) RmPathsFromContainer(ctx context.Context, cID container.ID, paths []PathMapping) error {
	if len(paths) == 0 {
		return nil
	}

	logger.Get(ctx).Debugf("Deleting %d files from container: %s", len(paths), cID.ShortStr())

	out := bytes.NewBuffer(nil)
	err := r.dCli.ExecInContainer(ctx, cID, model.Cmd{Argv: makeRmCmd(paths)}, out)
	if err != nil {
		if docker.IsExitError(err) {
			return fmt.Errorf("Error deleting files from container: %s", out.String())
		}
		return errors.Wrap(err, "Error deleting files from container")
	}
	return nil
}

func makeRmCmd(paths []PathMapping) []string {
	cmd := []string{"rm", "-rf"}
	for _, p := range paths {
		cmd = append(cmd, p.ContainerPath)
	}
	return cmd
}
