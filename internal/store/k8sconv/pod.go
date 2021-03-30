package k8sconv

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Convert a Kubernetes Pod into a list if simpler Container models to store in the engine state.
func PodContainers(ctx context.Context, pod *v1.Pod, containerStatuses []v1.ContainerStatus) []store.Container {
	result := make([]store.Container, 0, len(containerStatuses))
	for _, cStatus := range containerStatuses {
		c, err := ContainerForStatus(ctx, pod, cStatus)
		if err != nil {
			logger.Get(ctx).Debugf("%s", err.Error())
			continue
		}

		if !c.Empty() {
			result = append(result, c)
		}
	}
	return result
}

// Convert a Kubernetes Pod and ContainerStatus into a simpler Container model to store in the engine state.
func ContainerForStatus(ctx context.Context, pod *v1.Pod, cStatus v1.ContainerStatus) (store.Container, error) {
	cName := k8s.ContainerNameFromContainerStatus(cStatus)

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		return store.Container{}, errors.Wrap(err, "Error parsing container ID")
	}

	cRef, err := container.ParseNamed(cStatus.Image)
	if err != nil {
		return store.Container{}, errors.Wrap(err, "Error parsing container image ID")

	}

	ports := make([]int32, 0)
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	for _, cPort := range cSpec.Ports {
		ports = append(ports, cPort.ContainerPort)
	}

	isRunning := false
	if cStatus.State.Running != nil && !cStatus.State.Running.StartedAt.IsZero() {
		isRunning = true
	}

	isTerminated := false
	if cStatus.State.Terminated != nil && !cStatus.State.Terminated.StartedAt.IsZero() {
		isTerminated = true
	}

	return store.Container{
		Name:       cName,
		ID:         cID,
		Ports:      ports,
		Ready:      cStatus.Ready,
		Running:    isRunning,
		Terminated: isTerminated,
		ImageRef:   cRef,
		Restarts:   int(cStatus.RestartCount),
		Status:     ContainerStatusToRuntimeState(cStatus),
	}, nil
}

func ContainerStatusToRuntimeState(status v1.ContainerStatus) model.RuntimeStatus {
	state := status.State
	if state.Terminated != nil {
		if state.Terminated.ExitCode == 0 {
			return model.RuntimeStatusOK
		} else {
			return model.RuntimeStatusError
		}
	}

	if state.Waiting != nil {
		if ErrorWaitingReasons[state.Waiting.Reason] {
			return model.RuntimeStatusError
		}
		return model.RuntimeStatusPending
	}

	if state.Running != nil {
		return model.RuntimeStatusOK
	}

	return model.RuntimeStatusUnknown
}

var ErrorWaitingReasons = map[string]bool{
	"CrashLoopBackOff":  true,
	"ErrImagePull":      true,
	"ImagePullBackOff":  true,
	"RunContainerError": true,
	"StartError":        true,
	"Error":             true,
}
