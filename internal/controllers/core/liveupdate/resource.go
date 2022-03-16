package liveupdate

import (
	"time"

	"github.com/docker/distribution/reference"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Helper interface for live-updating different kinds of resources.
//
// This interface uses the language "pods", even though only Kubernetes
// has pods. For other kinds of orchestrator, just use whatever kind
// of workload primitive fits best.
type luResource interface {
	// The time to start syncing changes from.
	bestStartTime() time.Time

	// The names of any pods, for tracking state.
	podNames() []types.NamespacedName

	// An iterator for visiting each container.
	visitSelectedContainers(visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool)
}

type luK8sResource struct {
	selector *v1alpha1.LiveUpdateKubernetesSelector
	res      *k8sconv.KubernetesResource
}

func (r *luK8sResource) bestStartTime() time.Time {
	if r.res.ApplyStatus != nil {
		return r.res.ApplyStatus.LastApplyStartTime.Time
	}

	startTime := time.Time{}
	for _, pod := range r.res.FilteredPods {
		if startTime.IsZero() || (!pod.CreatedAt.IsZero() && pod.CreatedAt.Time.Before(startTime)) {
			startTime = pod.CreatedAt.Time
		}
	}
	return startTime
}

func (r *luK8sResource) podNames() []types.NamespacedName {
	result := []types.NamespacedName{}
	for _, pod := range r.res.FilteredPods {
		result = append(result, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace})
	}
	return result
}

// Visit all selected containers.
func (r *luK8sResource) visitSelectedContainers(
	visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool) {
	for _, pod := range r.res.FilteredPods {
		for _, c := range pod.Containers {
			if c.Name == "" {
				// ignore any blatantly invalid containers
				continue
			}

			// LiveUpdateKubernetesSelector must specify EITHER image OR container name
			if r.selector.Image != "" {
				imageRef, err := container.ParseNamed(c.Image)
				if err != nil || imageRef == nil || r.selector.Image != reference.FamiliarName(imageRef) {
					continue
				}
			} else if r.selector.ContainerName != c.Name {
				continue
			}
			stop := visit(pod, c)
			if stop {
				return
			}
		}
	}
}

// We model the DockerCompose resource as a single-container pod with a
// name equal to the container id.
type luDCResource struct {
	selector *v1alpha1.LiveUpdateDockerComposeSelector
	res      *v1alpha1.DockerComposeService
}

func (r *luDCResource) bestStartTime() time.Time {
	return r.res.Status.LastApplyStartTime.Time
}

// In DockerCompose, we treat every container as a single-container pod.
func (r *luDCResource) podNames() []types.NamespacedName {
	result := []types.NamespacedName{}
	if r.res.Status.ContainerID != "" {
		result = append(result, types.NamespacedName{Name: r.res.Status.ContainerID})
	}
	return result
}

// Visit all selected containers.
func (r *luDCResource) visitSelectedContainers(
	visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool) {
	cID := r.res.Status.ContainerID
	state := r.res.Status.ContainerState
	if cID != "" && state != nil {
		// In DockerCompose, we treat every container as a single-container pod.
		pod := v1alpha1.Pod{
			Name: cID,
		}
		var waiting *v1alpha1.ContainerStateWaiting
		var running *v1alpha1.ContainerStateRunning
		var terminated *v1alpha1.ContainerStateTerminated
		switch state.Status {
		case dockercompose.ContainerStatusCreated,
			dockercompose.ContainerStatusPaused,
			dockercompose.ContainerStatusRestarting:
			waiting = &v1alpha1.ContainerStateWaiting{Reason: state.Status}
		case dockercompose.ContainerStatusRunning:
			running = &v1alpha1.ContainerStateRunning{
				StartedAt: apis.NewTime(state.StartedAt.Time),
			}
		case dockercompose.ContainerStatusRemoving,
			dockercompose.ContainerStatusExited,
			dockercompose.ContainerStatusDead:
			terminated = &v1alpha1.ContainerStateTerminated{
				ExitCode:   state.ExitCode,
				Reason:     state.Status,
				StartedAt:  apis.NewTime(state.StartedAt.Time),
				FinishedAt: apis.NewTime(state.FinishedAt.Time),
			}
		}
		c := v1alpha1.Container{
			Name: cID,
			ID:   cID,
			State: v1alpha1.ContainerState{
				Waiting:    waiting,
				Running:    running,
				Terminated: terminated,
			},
			Ready: running != nil,
		}
		visit(pod, c)
	}
}
