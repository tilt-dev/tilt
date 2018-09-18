package build

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
)

type ContainerResolver struct {
	dcli docker.DockerClient
}

func NewContainerResolver(dcli docker.DockerClient) *ContainerResolver {
	return &ContainerResolver{dcli: dcli}
}

// containerIdForPod looks for the container ID associated with the pod.
// Expects to find exactly one matching container -- if not, return error.
// TODO: support multiple matching container IDs, i.e. restarting multiple containers per pod
func (r *ContainerResolver) ContainerIDForPod(ctx context.Context, podName k8s.PodID) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ContainerResolver-containerIdForPod")
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
		if c.Command != k8s.PauseCmd {
			return k8s.ContainerID(c.ID), nil
		}
	}

	// What?? No actual matches??!
	return "", fmt.Errorf("no matching non-'/pause' containers")
}
