package liveupdates

import (
	"fmt"

	"github.com/docker/distribution/reference"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dcconv"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func AllRunningContainers(mt *store.ManifestTarget, state *store.EngineState) []Container {
	if mt.Manifest.IsDC() {
		return RunningContainersForDC(mt.State.DockerResource())
	}

	var result []Container
	for _, iTarget := range mt.Manifest.ImageTargets {
		selector := iTarget.LiveUpdateSpec.Selector
		if mt.Manifest.IsK8s() && selector.Kubernetes != nil {
			cInfos, err := RunningContainersForOnePod(
				selector.Kubernetes,
				state.KubernetesResources[mt.Manifest.Name.String()])
			if err != nil {
				// HACK(maia): just don't collect container info for targets running
				// more than one pod -- we don't support LiveUpdating them anyway,
				// so no need to monitor those containers for crashes.
				continue
			}
			result = append(result, cInfos...)
		}
	}
	return result
}

func RunningContainers(selector *v1alpha1.LiveUpdateKubernetesSelector, k8sResource *k8sconv.KubernetesResource, dResource *dcconv.DockerResource) ([]Container, error) {
	if selector != nil && k8sResource != nil {
		return RunningContainersForOnePod(selector, k8sResource)
	}
	if dResource != nil {
		return RunningContainersForDC(dResource), nil
	}
	return nil, nil
}

// If all containers running the given image are ready, returns info for them.
// (If this image is running on multiple pods, return an error.)
func RunningContainersForOnePod(selector *v1alpha1.LiveUpdateKubernetesSelector, resource *k8sconv.KubernetesResource) ([]Container, error) {
	if selector == nil || resource == nil {
		return nil, nil
	}

	activePods := []v1alpha1.Pod{}
	for _, p := range resource.FilteredPods {
		// Ignore completed pods.
		if p.Phase == string(v1.PodSucceeded) || p.Phase == string(v1.PodFailed) {
			continue
		}
		activePods = append(activePods, p)
	}

	if len(activePods) == 0 {
		return nil, nil
	}
	if len(activePods) > 1 {
		return nil, fmt.Errorf("can only get container info for a single pod; image target %s has %d pods", selector.Image, len(resource.FilteredPods))
	}

	pod := activePods[0]
	var containers []Container
	for _, c := range pod.Containers {
		// Only return containers matching our image
		imageRef, err := container.ParseNamed(c.Image)
		if err != nil || imageRef == nil || selector.Image != reference.FamiliarName(imageRef) {
			continue
		}
		if c.ID == "" || c.Name == "" || c.State.Running == nil {
			// If we're missing any relevant info for this container, OR if the
			// container isn't running, we can't update it in place.
			// (Since we'll need to fully rebuild this image, we shouldn't bother
			// in-place updating ANY containers on this pod -- they'll all
			// be recreated when we image build. So don't return ANY Containers.)
			return nil, nil
		}
		containers = append(containers, Container{
			PodID:         k8s.PodID(pod.Name),
			ContainerID:   container.ID(c.ID),
			ContainerName: container.Name(c.Name),
			Namespace:     k8s.Namespace(pod.Namespace),
		})
	}

	return containers, nil
}

func RunningContainersForDC(dr *dcconv.DockerResource) []Container {
	if dr == nil || dr.ContainerID == "" {
		return nil
	}
	return []Container{
		Container{ContainerID: container.ID(dr.ContainerID)},
	}
}

// Information describing a single running & ready container
type Container struct {
	PodID         k8s.PodID
	ContainerID   container.ID
	ContainerName container.Name
	Namespace     k8s.Namespace
}

func (c Container) Empty() bool {
	return c == Container{}
}

func IDsForContainers(infos []Container) []container.ID {
	ids := make([]container.ID, len(infos))
	for i, info := range infos {
		ids[i] = info.ContainerID
	}
	return ids
}
