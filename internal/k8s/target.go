package k8s

import (
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/model"
)

func NewTarget(
	name model.TargetName,
	entities []K8sEntity,
	portForwards []model.PortForward,
	extraPodSelectors []labels.Selector,
	dependencyIDs []model.TargetID) (model.K8sTarget, error) {
	yaml, err := SerializeSpecYAML(entities)
	if err != nil {
		return model.K8sTarget{}, err
	}

	var resourceNames []string
	for _, e := range entities {
		resourceNames = append(resourceNames, e.ResourceName())
	}

	return model.K8sTarget{
		Name:              name,
		YAML:              yaml,
		ResourceNames:     resourceNames,
		PortForwards:      portForwards,
		ExtraPodSelectors: extraPodSelectors,
	}.WithDependencyIDs(dependencyIDs), nil
}

func NewK8sOnlyManifest(name model.ManifestName, entities []K8sEntity) (model.Manifest, error) {
	kTarget, err := NewTarget(name.TargetName(), entities, nil, nil, nil)
	if err != nil {
		return model.Manifest{}, err
	}
	return model.Manifest{Name: name}.WithDeployTarget(kTarget), nil
}

func NewK8sOnlyManifestForTesting(yaml string, entityNames []string) model.Manifest {
	return model.Manifest{Name: model.UnresourcedYAMLManifestName}.
		WithDeployTarget(model.K8sTarget{
			Name:          model.UnresourcedYAMLManifestName.TargetName(),
			YAML:          yaml,
			ResourceNames: entityNames,
		})
}
