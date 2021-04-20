package k8s

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
)

const ContainerIDPrefix = "docker://"

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

// FixContainerStatusImagesNoMutation is the same as FixContainerStatusImages but it does not mutate the input.
// It instead makes a deep copy and returns that with the updated status.
// It should be used over FixContainerStatusImages when the source of the pod is shared such as an informer.
func FixContainerStatusImagesNoMutation(pod *v1.Pod) *v1.Pod {
	pod = pod.DeepCopy()
	FixContainerStatusImages(pod)
	return pod
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

func ContainerSpecOf(pod *v1.Pod, status v1.ContainerStatus) v1.Container {
	for _, spec := range pod.Spec.Containers {
		if spec.Name == status.Name {
			return spec
		}
	}
	return v1.Container{}
}
