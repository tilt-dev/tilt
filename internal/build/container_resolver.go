package build

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
)

// A magic constant. If the docker client returns this constant, we always match
// even if the container doesn't have the correct image name.
const MagicTestContainerID = "tilt-testcontainer"

const containerUpTimeout = 10 * time.Second

type ContainerResolver struct {
	dcli docker.DockerClient
}

func NewContainerResolver(dcli docker.DockerClient) *ContainerResolver {
	return &ContainerResolver{dcli: dcli}
}

// containerIdForPod looks for the container ID associated with the pod and image ID
func (r *ContainerResolver) ContainerIDForPod(ctx context.Context, podName k8s.PodID, image reference.NamedTagged) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ContainerResolver-containerIdForPod")
	defer span.Finish()

	// Right now, we poll the pod until the container comes up. We give up after a timeout.
	// In the future, we might want to be more clever about asking k8s if the container
	// is in the process of coming up, or if we're waiting in vain.
	ctx, cancel := context.WithTimeout(ctx, containerUpTimeout)
	defer cancel()

	var lastErr error
	for ctx.Err() == nil {
		id, err := r.containerIDForPodHelper(ctx, podName, image)
		if err == nil {
			return id, nil
		}

		_, isContainerNotFound := err.(containerNotFound)
		if !isContainerNotFound {
			return "", err
		}

		lastErr = err
		time.Sleep(containerUpTimeout / 10)
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", ctx.Err()
}

func (r *ContainerResolver) containerIDForPodHelper(ctx context.Context, podName k8s.PodID, image reference.NamedTagged) (k8s.ContainerID, error) {

	a := filters.NewArgs()
	a.Add("name", string(podName))
	listOpts := types.ContainerListOptions{Filters: a}

	containers, err := r.dcli.ContainerList(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("getting containers: %v", err)
	}

	if len(containers) == 0 {
		return "", containerNotFound{
			Message: fmt.Sprintf("no matching containers found in pod: %s", podName),
		}
	}

	for _, c := range containers {
		if c.ID == MagicTestContainerID {
			return k8s.ContainerID(c.ID), nil
		}

		dig, err := digest.Parse(c.ImageID)
		if err != nil {
			logger.Get(ctx).Debugf("Skipping malformed digest %q: %v", c.ImageID, err)
			continue
		}
		if digestMatchesRef(image, dig) {
			return k8s.ContainerID(c.ID), nil
		}
	}

	// TODO(nick): We should have a way to wait if the container
	// simply hasn't materialized yet.
	return "", containerNotFound{
		Message: fmt.Sprintf("no containers matching: %s", image),
	}
}

type containerNotFound struct {
	Message string
}

func (e containerNotFound) Error() string {
	return e.Message
}
