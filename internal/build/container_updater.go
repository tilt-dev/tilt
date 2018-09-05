package build

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

const pauseCmd = "/pause"

type ContainerUpdater struct {
	dcli DockerClient
}

func NewContainerUpdater(dcli DockerClient) *ContainerUpdater {
	return &ContainerUpdater{dcli: dcli}
}

func (r *ContainerUpdater) UpdateInContainer(ctx context.Context, cID k8s.ContainerID, paths []pathMapping, steps []model.Cmd) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-UpdateInContainer")
	defer span.Finish()

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
	ab := newArchiveBuilder()
	err = ab.archivePathsIfExist(ctx, paths)
	if err != nil {
		return fmt.Errorf("archivePathsIfExists: %v", err)
	}
	archive, err := ab.bytesBuffer()
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
		err = r.dcli.ExecInContainer(ctx, cID, s)
		if err != nil {
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

// containerIdForPod looks for the container ID associated with the pod.
// Expects to find exactly one matching container -- if not, return error.
// TODO: support multiple matching container IDs, i.e. restarting multiple containers per pod
func (r *ContainerUpdater) ContainerIDForPod(ctx context.Context, podName k8s.PodID) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-containerIdForPod")
	defer span.Finish()

	a := filters.NewArgs()
	a.Add("name", string(podName))
	listOpts := types.ContainerListOptions{Filters: a}

	containers, err := r.dcli.ContainerList(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("getting containers: %v", err)
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("no containers found with name %s", podName)
	}

	// On GKE, we expect there to be one real match and one spurious match -- a
	// container running "/pause" (see: http://bit.ly/2BVtBXB); filter it out.
	if len(containers) > 2 {
		var ids []string
		for _, c := range containers {
			ids = append(ids, k8s.ContainerID(c.ID).ShortStr())
		}
		return "", fmt.Errorf("too many matching containers (%v)", ids)
	}

	for _, c := range containers {
		// TODO(maia): more robust check here (what if user is running a container with "/pause" command?!)
		if c.Command != pauseCmd {
			return k8s.ContainerID(c.ID), nil
		}
	}

	// What?? No actual matches??!
	return "", fmt.Errorf("no matching non-'/pause' containers")
}

func (r *ContainerUpdater) RmPathsFromContainer(ctx context.Context, cID k8s.ContainerID, paths []pathMapping) error {
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
