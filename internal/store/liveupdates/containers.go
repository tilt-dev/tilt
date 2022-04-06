package liveupdates

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// If all containers running the given image are ready, returns info for them.
// (If this image is running on multiple pods, return an error.)
func RunningContainersForOnePod(
	selector *v1alpha1.LiveUpdateKubernetesSelector,
	resource *k8sconv.KubernetesResource,
	imageMap *v1alpha1.ImageMap,
) ([]Container, error) {
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
		if !liveupdate.KubernetesSelectorMatchesContainer(c, selector, imageMap) {
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

func RunningContainersForDC(dcs *v1alpha1.DockerComposeService) []Container {
	if dcs == nil || dcs.Status.ContainerID == "" {
		return nil
	}
	return []Container{
		Container{ContainerID: container.ID(dcs.Status.ContainerID)},
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

func (c Container) DisplayName() string {
	if c.PodID == "" {
		if c.ContainerName == "" {
			return c.ContainerID.ShortStr()
		}
		return c.ContainerName.String()
	}

	return fmt.Sprintf("%s/%s", c.PodID, c.ContainerName)
}
