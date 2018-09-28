package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/logger"
	"k8s.io/api/core/v1"
)

const containerIDPrefix = "docker://"

func WaitForContainerReady(ctx context.Context, client Client, pod *v1.Pod, ref reference.Named) (ContainerID, error) {
	cID, err := waitForContainerReadyHelper(pod, ref)
	if err != nil {
		return "", err
	} else if cID != "" {
		return cID, nil
	}

	watch, err := client.WatchPod(ctx, pod)
	if err != nil {
		return "", errors.Wrap(err, "WaitForContainerReady")
	}
	defer watch.Stop()

	for true {
		select {
		case <-ctx.Done():
			return "", errors.Wrap(ctx.Err(), "WaitForContainerReady")
		case event, ok := <-watch.ResultChan():
			if !ok {
				return "", fmt.Errorf("Container watch closed: %s", ref)
			}

			obj := event.Object
			pod, ok := obj.(*v1.Pod)
			if !ok {
				logger.Get(ctx).Debugf("Unexpected watch notification: %T", obj)
				continue
			}

			cID, err := waitForContainerReadyHelper(pod, ref)
			if err != nil {
				return "", err
			} else if cID != "" {
				return cID, nil
			}
		}
	}
	panic("WaitForContainerReady") // should never reach this state
}

func waitForContainerReadyHelper(pod *v1.Pod, ref reference.Named) (ContainerID, error) {
	cStatus, err := ContainerMatching(pod, ref)
	if err != nil {
		return "", errors.Wrap(err, "WaitForContainerReadyHelper")
	}

	if IsContainerExited(pod.Status, cStatus) {
		return "", fmt.Errorf("Container will never be ready: %s", ref)
	}

	if cStatus.Ready {
		return ContainerIDFromContainerStatus(cStatus)
	}

	return "", nil
}

// If true, this means the container is gone and will never recover.
func IsContainerExited(pod v1.PodStatus, container v1.ContainerStatus) bool {
	if pod.Phase == v1.PodSucceeded || pod.Phase == v1.PodFailed {
		return true
	}

	if container.State.Terminated != nil {
		return true
	}

	return false
}

func ContainerMatching(pod *v1.Pod, ref reference.Named) (v1.ContainerStatus, error) {
	for _, c := range pod.Status.ContainerStatuses {
		cRef, err := reference.ParseNormalizedNamed(c.Image)
		if err != nil {
			return v1.ContainerStatus{}, errors.Wrap(err, "ContainerMatching")
		}

		if cRef.Name() == ref.Name() {
			return c, nil
		}
	}
	return v1.ContainerStatus{}, nil
}

func ContainerIDFromContainerStatus(status v1.ContainerStatus) (ContainerID, error) {
	id := status.ContainerID
	if !strings.HasPrefix(id, containerIDPrefix) {
		return "", fmt.Errorf("Malformed container ID: %s", id)
	}
	return ContainerID(id[len(containerIDPrefix):]), nil
}
