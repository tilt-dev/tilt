package engine

import (
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/model"
)

func ParseYAMLFromManifests(manifests ...model.Manifest) ([]k8s.K8sEntity, error) {
	var allEntities []k8s.K8sEntity
	for _, m := range manifests {
		if !m.IsK8s() {
			continue
		}
		entities, err := k8s.ParseYAMLFromString(m.K8sTarget().YAML)
		if err != nil {
			return nil, err
		}

		allEntities = append(allEntities, entities...)
	}
	return allEntities, nil
}
