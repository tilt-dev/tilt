package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/opencontainers/go-digest"
	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

const pauseCmd = "/pause"

type ContainerUpdater interface {
	UpdateInContainer(ctx context.Context, paths []pathMapping, steps []model.Cmd) error
}

var _ ContainerUpdater = &containerUpdater{}

type containerUpdater struct {
	dcli DockerClient

	// TODO(maia): actually, maybe remove this and pass pod (or container) into UpdateInContainer
	pod string // TODO(maia): support multiple pods -- for now, PoC with one
}

func (r *containerUpdater) UpdateInContainer(ctx context.Context, paths []pathMapping, steps []model.Cmd) error {
	cID, err := r.containerIdForPod(ctx)
	if err != nil {
		return err
	}

	// rm files from container
	toRemove, err := missingLocalPaths(ctx, paths)
	if err != nil {
		return fmt.Errorf("missingLocalPaths: %v", err)
	}

	err = r.RmPathsFromContainer(ctx, cID, toRemove)
	if err != nil {
		return fmt.Errorf("RmPathsFromContainer: %v", err)
	}

	// copy files to container
	archive, err := ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return err
	}

	logger.Get(ctx).Debugf("Copying files to container: %s", cID.ShortStr())

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = r.dcli.CopyToContainer(ctx, cID.String(), "/", bytes.NewReader(archive.Bytes()),
		types.CopyToContainerOptions{})
	if err != nil {
		return err
	}

	// Exec steps on container
	for _, s := range steps {
		err = r.dcli.ExecInContainer(ctx, cID, s)
		if err != nil {
			return fmt.Errorf("executing step %v on container %s: %v", s.Argv, cID.ShortStr(), err)
		}
	}

	// TODO(maia): restart container(s)

	return nil
}

// TODO(maia): reorg tar funcs in a more logical way
func ArchivePathsIfExist(ctx context.Context, paths []pathMapping) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func() {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %s", err.Error())
		}
	}()
	err := archivePathsIfExist(ctx, tw, paths)
	if err != nil {
		return nil, fmt.Errorf("archivePathsIfExists: %v", err)
	}
	return buf, nil
}

func (r *containerUpdater) PushDocker(ctx context.Context, name reference.NamedTagged) (reference.NamedTagged, error) {
	return nil, fmt.Errorf("PushDocker definitely not implemented on containerUpdater")
}
func (r *containerUpdater) TagDocker(ctx context.Context, name reference.Named, dig digest.Digest) (reference.NamedTagged, error) {
	return nil, fmt.Errorf("TagDocker definitely not implemented on containerUpdater")
}

// containerIdForPod looks for the container ID associated with the pod.
// Expects to find exactly one matching container -- if not, return error.
// TODO: support multiple matching container IDs, i.e. restarting multiple containers per pod
func (r *containerUpdater) containerIdForPod(ctx context.Context) (containerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-containerIdForPod")
	defer span.Finish()

	a := filters.NewArgs()
	a.Add("name", r.pod)
	listOpts := types.ContainerListOptions{Filters: a}

	containers, err := r.dcli.ContainerList(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("getting containers: %v", err)
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("no containers found with name %s", r.pod)
	}

	// On GKE, we expect there to be one real match and one spurious match -- a
	// container running "/pause" (see: http://bit.ly/2BVtBXB); filter it out.
	if len(containers) > 2 {
		var ids []string
		for _, c := range containers {
			ids = append(ids, containerID(c.ID).ShortStr())
		}
		return "", fmt.Errorf("too many matching containers (%v)", ids)
	}

	for _, c := range containers {
		// TODO(maia): more robust check here (what if user is running a container with "/pause" command?!)
		if c.Command != pauseCmd {
			return containerID(c.ID), nil
		}
	}

	// What?? No actual matches??!
	return "", fmt.Errorf("no matching non-'/pause' containers")
}

func (r *containerUpdater) RmPathsFromContainer(ctx context.Context, cID containerID, paths []pathMapping) error {
	if len(paths) == 0 {
		return nil
	}

	logger.Get(ctx).Debugf("Deleting %d files from container: %s", len(paths), cID.ShortStr())

	return r.dcli.ExecInContainer(ctx, cID, model.Cmd{Argv: makeRmCmd(paths)})
}

func makeRmCmd(paths []pathMapping) []string {
	cmd := []string{"rm", "-rf"}
	for _, p := range paths {
		cmd = append(cmd, p.ContainerPath)
	}
	return cmd
}
