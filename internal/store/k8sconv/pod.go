package k8sconv

import (
	"context"
	"fmt"

	"github.com/tilt-dev/tilt/pkg/model/logstore"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func Pod(ctx context.Context, pod *v1.Pod) *v1alpha1.Pod {
	podInfo := &v1alpha1.Pod{
		Name:           pod.Name,
		Namespace:      pod.Namespace,
		CreatedAt:      *pod.CreationTimestamp.DeepCopy(),
		Phase:          string(pod.Status.Phase),
		Deleting:       pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero(),
		Conditions:     PodConditions(pod.Status.Conditions),
		InitContainers: PodContainers(ctx, pod, pod.Status.InitContainerStatuses),
		Containers:     PodContainers(ctx, pod, pod.Status.ContainerStatuses),

		PodTemplateSpecHash: pod.Labels[k8s.TiltPodTemplateHashLabel],
		Status:              PodStatusToString(*pod),
		Errors:              PodStatusErrorMessages(*pod),
	}
	return podInfo
}

func PodConditions(conditions []v1.PodCondition) []v1alpha1.PodCondition {
	result := make([]v1alpha1.PodCondition, 0, len(conditions))
	for _, c := range conditions {
		condition := v1alpha1.PodCondition{
			Type:               string(c.Type),
			Status:             string(c.Status),
			LastTransitionTime: *c.LastTransitionTime.DeepCopy(),
			Reason:             c.Reason,
			Message:            c.Message,
		}
		result = append(result, condition)
	}
	return result
}

// Convert a Kubernetes Pod into a list of simpler Container models to store in the engine state.
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

func ContainerStatusToRuntimeState(status v1alpha1.Container) v1alpha1.RuntimeStatus {
	state := status.State
	if state.Terminated != nil {
		if state.Terminated.ExitCode == 0 {
			return v1alpha1.RuntimeStatusOK
		} else {
			return v1alpha1.RuntimeStatusError
		}
	}

	if state.Waiting != nil {
		if ErrorWaitingReasons[state.Waiting.Reason] {
			return v1alpha1.RuntimeStatusError
		}
		return v1alpha1.RuntimeStatusPending
	}

	// TODO(milas): this should really consider status.Ready
	if state.Running != nil {
		return v1alpha1.RuntimeStatusOK
	}

	return v1alpha1.RuntimeStatusUnknown
}

var ErrorWaitingReasons = map[string]bool{
	"CrashLoopBackOff":  true,
	"ErrImagePull":      true,
	"ImagePullBackOff":  true,
	"RunContainerError": true,
	"StartError":        true,
	"Error":             true,
}

// SpanIDForPod creates a span ID for a given pod associated with a manifest.
//
// Generally, a given Pod is only referenced by a single manifest, but there are
// rare occasions where it can be referenced by multiple. If the span ID is not
// unique between them, things will behave erratically.
func SpanIDForPod(mn model.ManifestName, podID k8s.PodID) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("pod:%s:%s", mn.String(), podID))
}

// copied from https://github.com/kubernetes/kubernetes/blob/aedeccda9562b9effe026bb02c8d3c539fc7bb77/pkg/kubectl/resource_printer.go#L692-L764
// to match the status column of `kubectl get pods`
func PodStatusToString(pod v1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	for i, container := range pod.Status.InitContainerStatuses {
		state := container.State

		switch {
		case state.Terminated != nil && state.Terminated.ExitCode == 0:
			continue
		case state.Terminated != nil:
			// initialization is failed
			if len(state.Terminated.Reason) == 0 {
				if state.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", state.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", state.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + state.Terminated.Reason
			}
		case state.Waiting != nil && len(state.Waiting.Reason) > 0 && state.Waiting.Reason != "PodInitializing":
			reason = "Init:" + state.Waiting.Reason
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
		}
		break
	}

	if isPodStillInitializing(pod) {
		return reason
	}

	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]
		state := container.State

		if state.Waiting != nil && state.Waiting.Reason != "" {
			reason = state.Waiting.Reason
		} else if state.Terminated != nil && state.Terminated.Reason != "" {
			reason = state.Terminated.Reason
		} else if state.Terminated != nil && state.Terminated.Reason == "" {
			if state.Terminated.Signal != 0 {
				reason = fmt.Sprintf("Signal:%d", state.Terminated.Signal)
			} else {
				reason = fmt.Sprintf("ExitCode:%d", state.Terminated.ExitCode)
			}
		}
	}

	return reason
}

// Pull out interesting error messages from the pod status
func PodStatusErrorMessages(pod v1.Pod) []string {
	result := []string{}
	if isPodStillInitializing(pod) {
		for _, container := range pod.Status.InitContainerStatuses {
			result = append(result, containerStatusErrorMessages(container)...)
		}
	}
	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]
		result = append(result, containerStatusErrorMessages(container)...)
	}
	return result
}

func containerStatusErrorMessages(container v1.ContainerStatus) []string {
	result := []string{}
	state := container.State
	if state.Waiting != nil {
		lastState := container.LastTerminationState
		if lastState.Terminated != nil &&
			lastState.Terminated.ExitCode != 0 &&
			lastState.Terminated.Message != "" {
			result = append(result, lastState.Terminated.Message)
		}

		// If we're in an error mode, also include the error message.
		// Many error modes put important information in the error message,
		// like when the pod will get rescheduled.
		if state.Waiting.Message != "" && ErrorWaitingReasons[state.Waiting.Reason] {
			result = append(result, state.Waiting.Message)
		}
	} else if state.Terminated != nil &&
		state.Terminated.ExitCode != 0 &&
		state.Terminated.Message != "" {
		result = append(result, state.Terminated.Message)
	}

	return result
}

func isPodStillInitializing(pod v1.Pod) bool {
	for _, container := range pod.Status.InitContainerStatuses {
		state := container.State
		isFinished := state.Terminated != nil && state.Terminated.ExitCode == 0
		if !isFinished {
			return true
		}
	}
	return false
}
