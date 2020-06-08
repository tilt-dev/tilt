package sidecar

import (
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
)

// Inject the synclet into any Pod
func InjectSyncletSidecar(entity k8s.K8sEntity, selector container.RefSelector, container SyncletContainer) (k8s.K8sEntity, bool, error) {
	entity = entity.DeepCopy()

	pods, err := k8s.ExtractPods(&entity)
	if err != nil {
		return k8s.K8sEntity{}, false, err
	}

	replaced := false
	for _, pod := range pods {
		ok, err := k8s.PodContainsRef(*pod, selector)
		if err != nil {
			return k8s.K8sEntity{}, false, err
		}

		if !ok {
			continue
		}

		replaced = true
		vol := SyncletVolume.DeepCopy()
		pod.Volumes = append(pod.Volumes, *vol)

		container := (*v1.Container)(container).DeepCopy()
		pod.Containers = append(pod.Containers, *container)
	}
	return entity, replaced, nil
}
