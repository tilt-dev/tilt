package sidecar

import (
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
)

// Inject the synclet into any Pod
func InjectSyncletSidecar(entity k8s.K8sEntity, selector container.RefSelector) (k8s.K8sEntity, bool, error) {
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

		container := SyncletContainer.DeepCopy()
		pod.Containers = append(pod.Containers, *container)
	}
	return entity, replaced, nil
}
