package liveupdates

import (
	"sort"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func FakeKubernetesResource(image string, containers []Container) *k8sconv.KubernetesResource {
	r, err := k8sconv.NewKubernetesResource(FakeKubernetesDiscovery(image, containers), nil)
	if err != nil {
		panic(err)
	}
	return r
}

// Given the set of containers we want, create a fake KubernetesDiscovery
// with those containers running.
func FakeKubernetesDiscovery(image string, containers []Container) *v1alpha1.KubernetesDiscovery {
	podMap := map[string]*v1alpha1.Pod{}
	for _, c := range containers {
		pod, ok := podMap[string(c.PodID)]
		if !ok {
			pod = &v1alpha1.Pod{
				Name:      string(c.PodID),
				Namespace: string(c.Namespace),
				Phase:     "Running",
			}
			podMap[string(c.PodID)] = pod
		}

		pod.Containers = append(pod.Containers, v1alpha1.Container{
			ID:    string(c.ContainerID),
			Name:  string(c.ContainerName),
			Ready: true,
			Image: image,
			State: v1alpha1.ContainerState{
				Running: &v1alpha1.ContainerStateRunning{},
			},
		})
	}

	pods := []v1alpha1.Pod{}
	for _, p := range podMap {
		pods = append(pods, *p)
	}
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	return &v1alpha1.KubernetesDiscovery{
		Status: v1alpha1.KubernetesDiscoveryStatus{
			Pods: pods,
		},
	}
}
