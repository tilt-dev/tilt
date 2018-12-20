package engine

import (
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

func ParseYAMLFromManifests(manifests ...model.Manifest) ([]k8s.K8sEntity, error) {
	var allEntities []k8s.K8sEntity
	for _, m := range manifests {
		k8sInfo := m.K8sInfo()
		if k8sInfo.Empty() {
			continue
		}
		entities, err := k8s.ParseYAMLFromString(k8sInfo.YAML)
		if err != nil {
			return nil, err
		}

		allEntities = append(allEntities, entities...)
	}
	return allEntities, nil
}
