package liveupdate

import (
	"time"

	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
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

	// An iterator for visiting each container.
	visitSelectedContainers(visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool)
}

type luK8sResource struct {
	selector *v1alpha1.LiveUpdateKubernetesSelector
	res      *k8sconv.KubernetesResource
	im       *v1alpha1.ImageMap
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

// Visit all selected containers.
func (r *luK8sResource) visitSelectedContainers(
	visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool) {
	for _, pod := range r.res.FilteredPods {
		for _, c := range pod.Containers {
			if c.Name == "" {
				// ignore any blatantly invalid containers
				continue
			}
			if !liveupdate.KubernetesSelectorMatchesContainer(c, r.selector, r.im) {
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

// Visit all selected containers.
func (r *luDCResource) visitSelectedContainers(
	visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool) {
	cID := r.res.Status.ContainerID
	state := r.res.Status.ContainerState
	if cID != "" && state != nil {
		// In DockerCompose, we leave the pod empty.
		pod := v1alpha1.Pod{}
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
		cName := r.res.Status.ContainerName
		c := v1alpha1.Container{
			Name: cName,
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
