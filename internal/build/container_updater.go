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

func (r *ContainerUpdater) UpdateInContainer(ctx context.Context, cID container.ID, paths []PathMapping, filter model.PathMatcher, runs []model.Cmd, hotReload bool, w io.Writer) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-UpdateInContainer")
	defer span.Finish()
	l := logger.Get(ctx)

	// rm files from container
	toRemove, toArchive, err := MissingLocalPaths(ctx, paths)
	if err != nil {
		return errors.Wrap(err, "MissingLocalPaths")
	}

	if len(toRemove) > 0 {
		l.Infof("Deleting %d file(s) from container: %s", len(toRemove), cID.ShortStr())
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	err = r.RmPathsFromContainer(ctx, cID, toRemove)
	if err != nil {
		return errors.Wrap(err, "RmPathsFromContainer")
	}

	// copy files to container
	ab := NewArchiveBuilder(filter)
	err = ab.ArchivePathsIfExist(ctx, toArchive)
	if err != nil {
		return errors.Wrap(err, "archivePathsIfExists")
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return err
	}

	if len(toArchive) > 0 {
		l.Infof("Copying %d file(s) to container: %s", len(toArchive), cID.ShortStr())
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = r.dCli.CopyToContainerRoot(ctx, cID.String(), bytes.NewReader(archive.Bytes()))
	if err != nil {
		return err
	}

	// Exec run's on container
	for _, s := range runs {
		err = r.dCli.ExecInContainer(ctx, cID, s, w)
		if err != nil {
			return WrapContainerExecError(err, cID, s)
		}
	}

	if hotReload {
		l.Debugf("Hot reload on, skipping container restart: %s", cID.ShortStr())
		return nil
	}

	// Restart container so that entrypoint restarts with the updated files etc.
	l.Debugf("Restarting container: %s", cID.ShortStr())
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
