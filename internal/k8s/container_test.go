package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestFixContainerStatusImages(t *testing.T) {
	pod := fakePod(expectedPod, blorgDevImgStr)
	pod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				Name:  "default",
				Image: blorgDevImgStr + "v2",
				Ready: true,
			},
		},
	}

	assert.NotEqual(t,
		pod.Spec.Containers[0].Image,
		pod.Status.ContainerStatuses[0].Image)
	FixContainerStatusImages(pod)
	assert.Equal(t,
		pod.Spec.Containers[0].Image,
		pod.Status.ContainerStatuses[0].Image)
}

func TestFixContainerStatusImagesNoMutation(t *testing.T) {
	origPod := fakePod(expectedPod, blorgDevImgStr)
	origPod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				Name:  "default",
				Image: blorgDevImgStr + "v2",
				Ready: true,
			},
		},
	}

	assert.NotEqual(t,
		origPod.Spec.Containers[0].Image,
		origPod.Status.ContainerStatuses[0].Image)

	podCopy := origPod.DeepCopy()
	newPod := FixContainerStatusImagesNoMutation(origPod)

	assert.Equal(t, podCopy, origPod)
	assert.NotEqual(t, newPod, origPod)

	assert.NotEqual(t,
		origPod.Spec.Containers[0].Image,
		origPod.Status.ContainerStatuses[0].Image)

	assert.Equal(t,
		origPod.Spec.Containers[0].Image,
		newPod.Status.ContainerStatuses[0].Image)

	assert.Equal(t,
		newPod.Spec.Containers[0].Image,
		newPod.Status.ContainerStatuses[0].Image)
}
