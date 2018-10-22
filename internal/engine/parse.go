package engine

import (
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

func ParseYAMLFromManifests(manifests ...model.Manifest) ([]k8s.K8sEntity, error) {
	allEntities := []k8s.K8sEntity{}
	for _, m := range manifests {
		entities, err := k8s.ParseYAMLFromString(m.K8sYaml)
		if err != nil {
			return nil, err
		}

		allEntities = append(allEntities, entities...)
	}
	return allEntities, nil
}
