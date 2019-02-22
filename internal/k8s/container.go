package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/logger"
	"k8s.io/api/core/v1"
)

const ContainerIDPrefix = "docker://"

func WaitForContainerReady(ctx context.Context, client Client, pod *v1.Pod, ref reference.Named) (v1.ContainerStatus, error) {
	cStatus, err := waitForContainerReadyHelper(pod, ref)
	if err != nil {
		return v1.ContainerStatus{}, err
	} else if cStatus != (v1.ContainerStatus{}) {
		return cStatus, nil
	}

	watch, err := client.WatchPod(ctx, pod)
	if err != nil {
		return v1.ContainerStatus{}, errors.Wrap(err, "WaitForContainerReady")
	}
	defer watch.Stop()

	for true {
		select {
		case <-ctx.Done():
			return v1.ContainerStatus{}, errors.Wrap(ctx.Err(), "WaitForContainerReady")
		case event, ok := <-watch.ResultChan():
			if !ok {
				return v1.ContainerStatus{}, fmt.Errorf("Container watch closed: %s", ref)
			}

			obj := event.Object
			pod, ok := obj.(*v1.Pod)
			if !ok {
				logger.Get(ctx).Debugf("Unexpected watch notification: %T", obj)
				continue
			}

			cStatus, err := waitForContainerReadyHelper(pod, ref)
			if err != nil {
				return v1.ContainerStatus{}, err
			} else if cStatus != (v1.ContainerStatus{}) {
				return cStatus, nil
			}
		}
	}
	panic("WaitForContainerReady") // should never reach this state
}

func waitForContainerReadyHelper(pod *v1.Pod, ref reference.Named) (v1.ContainerStatus, error) {
	cStatus, err := ContainerMatching(pod, ref)
	if err != nil {
		return v1.ContainerStatus{}, errors.Wrap(err, "WaitForContainerReadyHelper")
	}

	unschedulable, msg := IsUnschedulable(pod.Status)
	if unschedulable {
		return v1.ContainerStatus{}, fmt.Errorf("Container will never be ready: %s", msg)
	}

	if IsContainerExited(pod.Status, cStatus) {
		return v1.ContainerStatus{}, fmt.Errorf("Container will never be ready: %s", ref)
	}

	if !cStatus.Ready {
		return v1.ContainerStatus{}, nil
	}

	return cStatus, nil
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

// Returns the error message if the pod is unschedulable
func IsUnschedulable(pod v1.PodStatus) (bool, string) {
	for _, cond := range pod.Conditions {
		if cond.Reason == v1.PodReasonUnschedulable {
			return true, cond.Message
		}
	}
	return false, ""
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

func ContainerIDFromContainerStatus(status v1.ContainerStatus) (container.ID, error) {
	id := status.ContainerID
	if id == "" {
		return "", nil
	}

	components := strings.SplitN(id, "://", 2)
	if len(components) != 2 {
		return "", fmt.Errorf("Malformed container ID: %s", id)
	}
	return container.ID(components[1]), nil
}

func ContainerNameFromContainerStatus(status v1.ContainerStatus) container.Name {
	return container.Name(status.Name)
}

func ContainerSpecOf(pod *v1.Pod, status v1.ContainerStatus) v1.Container {
	for _, spec := range pod.Spec.Containers {
		if spec.Name == status.Name {
			return spec
		}
	}
	return v1.Container{}
}
