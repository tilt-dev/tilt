package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const ContainerIDPrefix = "docker://"

func WaitForContainerReady(ctx context.Context, client Client, pod *v1.Pod, ref container.RefSelector) (v1.ContainerStatus, error) {
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

	for {
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

			FixContainerStatusImages(pod)

			cStatus, err := waitForContainerReadyHelper(pod, ref)
			if err != nil {
				return v1.ContainerStatus{}, err
			} else if cStatus != (v1.ContainerStatus{}) {
				return cStatus, nil
			}
		}
	}
}

func waitForContainerReadyHelper(pod *v1.Pod, ref container.RefSelector) (v1.ContainerStatus, error) {
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

// Kubernetes has a bug where the image ref in the container status
// can be wrong (though this does not mean the container is running
// unexpected code)
//
// Repro steps:
// 1) Create an image and give it two different tags (A and B)
// 2) Deploy Pods with both A and B in the pod spec
// 3) The PodStatus will choose A or B for both pods.
//
// More details here:
// https://github.com/kubernetes/kubernetes/issues/51017
//
// For Tilt, it's pretty important that the image tag is correct (for matching
// purposes). To work around this bug, we change the image reference in
// ContainerStatus to match the ContainerSpec.
func FixContainerStatusImages(pod *v1.Pod) {
	refsByContainerName := make(map[string]string)
	for _, c := range pod.Spec.Containers {
		if c.Name != "" {
			refsByContainerName[c.Name] = c.Image
		}
	}
	for i, cs := range pod.Status.ContainerStatuses {
		image, ok := refsByContainerName[cs.Name]
		if !ok {
			continue
		}

		cs.Image = image
		pod.Status.ContainerStatuses[i] = cs
	}
}

func ContainerMatching(pod *v1.Pod, ref container.RefSelector) (v1.ContainerStatus, error) {
	for _, c := range pod.Status.ContainerStatuses {
		cRef, err := container.ParseNamed(c.Image)
		if err != nil {
			return v1.ContainerStatus{}, errors.Wrap(err, "ContainerMatching")
		}

		if ref.Matches(cRef) {
			return c, nil
		}
	}
	return v1.ContainerStatus{}, nil
}

func ContainerIDFromContainerStatus(status v1.ContainerStatus) (container.ID, error) {
	id := status.ContainerID
	return NormalizeContainerID(id)
}

func NormalizeContainerID(id string) (container.ID, error) {
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
