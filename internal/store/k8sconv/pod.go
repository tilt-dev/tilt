package k8sconv

import (
	"context"
	"fmt"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Convert a Kubernetes Pod into a list if simpler Container models to store in the engine state.
func PodContainers(ctx context.Context, pod *v1.Pod, containerStatuses []v1.ContainerStatus) []v1alpha1.Container {
	result := make([]v1alpha1.Container, 0, len(containerStatuses))
	for _, cStatus := range containerStatuses {
		c, err := ContainerForStatus(pod, cStatus)
		if err != nil {
			logger.Get(ctx).Debugf("%s", err.Error())
			continue
		}

		if c.Name != "" {
			result = append(result, c)
		}
	}
	return result
}

// Convert a Kubernetes Pod and ContainerStatus into a simpler Container model to store in the engine state.
func ContainerForStatus(pod *v1.Pod, cStatus v1.ContainerStatus) (v1alpha1.Container, error) {
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	ports := make([]int32, len(cSpec.Ports))
	for i, cPort := range cSpec.Ports {
		ports[i] = cPort.ContainerPort
	}

	cID, err := k8s.NormalizeContainerID(cStatus.ContainerID)
	if err != nil {
		return v1alpha1.Container{}, fmt.Errorf("error parsing container ID: %w", err)
	}

	c := v1alpha1.Container{
		Name:     cStatus.Name,
		ID:       string(cID),
		Ready:    cStatus.Ready,
		Image:    cStatus.Image,
		Restarts: cStatus.RestartCount,
		State:    v1alpha1.ContainerState{},
		Ports:    ports,
	}

	if cStatus.State.Waiting != nil {
		c.State.Waiting = &v1alpha1.ContainerStateWaiting{
			Reason: cStatus.State.Waiting.Reason,
		}
	} else if cStatus.State.Running != nil {
		c.State.Running = &v1alpha1.ContainerStateRunning{
			StartedAt: *cStatus.State.Running.StartedAt.DeepCopy(),
		}
	} else if cStatus.State.Terminated != nil {
		c.State.Terminated = &v1alpha1.ContainerStateTerminated{
			StartedAt:  *cStatus.State.Terminated.StartedAt.DeepCopy(),
			FinishedAt: *cStatus.State.Terminated.FinishedAt.DeepCopy(),
			Reason:     cStatus.State.Terminated.Reason,
			ExitCode:   cStatus.State.Terminated.ExitCode,
		}
	}

	return c, nil
}

func ContainerStatusToRuntimeState(status v1alpha1.Container) model.RuntimeStatus {
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

	// TODO(milas): this should really consider status.Ready
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
