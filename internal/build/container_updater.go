package build

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type ContainerUpdater struct {
	dcli docker.DockerClient
}

func NewContainerUpdater(dcli docker.DockerClient) *ContainerUpdater {
	return &ContainerUpdater{dcli: dcli}
}

func (r *ContainerUpdater) UpdateInContainer(ctx context.Context, cID container.ContainerID, paths []pathMapping, filter model.PathMatcher, steps []model.Cmd, w io.Writer) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-UpdateInContainer")
	defer span.Finish()

	// rm files from container
	toRemove, err := MissingLocalPaths(ctx, paths)
	if err != nil {
		return fmt.Errorf("MissingLocalPaths: %v", err)
	}

	err = r.RmPathsFromContainer(ctx, cID, toRemove)
	if err != nil {
		return fmt.Errorf("RmPathsFromContainer: %v", err)
	}

	// copy files to container
	ab := NewArchiveBuilder(filter)
	err = ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return fmt.Errorf("archivePathsIfExists: %v", err)
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return err
	}

	logger.Get(ctx).Debugf("Copying files to container: %s", cID.ShortStr())

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = r.dcli.CopyToContainerRoot(ctx, cID.String(), bytes.NewReader(archive.Bytes()))
	if err != nil {
		return err
	}

	// Exec steps on container
	for _, s := range steps {
		err = r.dcli.ExecInContainer(ctx, cID, s, w)
		if err != nil {
			exitErr, isExitErr := err.(docker.ExitError)
			if isExitErr {
				return UserBuildFailure{ExitCode: exitErr.ExitCode}
			}
			return fmt.Errorf("executing step %v on container %s: %v", s.Argv, cID.ShortStr(), err)
		}
	}

	// Restart container so that entrypoint restarts with the updated files etc.
	err = r.dcli.ContainerRestartNoWait(ctx, cID.String())
	if err != nil {
		return fmt.Errorf("ContainerRestart: %v", err)
	}
	return nil
}

func (r *ContainerUpdater) RmPathsFromContainer(ctx context.Context, cID container.ContainerID, paths []pathMapping) error {
	if len(paths) == 0 {
		return nil
	}

	logger.Get(ctx).Debugf("Deleting %d files from container: %s", len(paths), cID.ShortStr())

	out := bytes.NewBuffer(nil)
	err := r.dcli.ExecInContainer(ctx, cID, model.Cmd{Argv: makeRmCmd(paths)}, out)
	if err != nil {
		if docker.IsExitError(err) {
			return fmt.Errorf("Error deleting files from container: %s", out.String())
		}
		return fmt.Errorf("Error deleting files from container: %v", err)
	}
	return nil
}

func makeRmCmd(paths []pathMapping) []string {
	cmd := []string{"rm", "-rf"}
	for _, p := range paths {
		cmd = append(cmd, p.ContainerPath)
	}
	return cmd
}
