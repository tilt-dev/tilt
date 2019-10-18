package k8s

import (
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/pkg/model"
)

func NewTarget(
	name model.TargetName,
	entities []K8sEntity,
	portForwards []model.PortForward,
	extraPodSelectors []labels.Selector,
	dependencyIDs []model.TargetID,
	refInjectCounts map[string]int) (model.K8sTarget, error) {
	sorted := SortedEntities(entities)
	yaml, err := SerializeSpecYAML(sorted)
	if err != nil {
		return model.K8sTarget{}, err
	}

	// Use a min component count of 2 for computing names,
	// so that the resource type appears
	displayNames := UniqueNames(sorted, 2)

	return model.K8sTarget{
		Name:              name,
		YAML:              yaml,
		PortForwards:      portForwards,
		ExtraPodSelectors: extraPodSelectors,
		DisplayNames:      displayNames,
	}.WithDependencyIDs(dependencyIDs).WithRefInjectCounts(refInjectCounts), nil
}

func NewK8sOnlyManifest(name model.ManifestName, entities []K8sEntity) (model.Manifest, error) {
	kTarget, err := NewTarget(name.TargetName(), entities, nil, nil, nil, nil)
	if err != nil {
		return model.Manifest{}, err
	}
	return model.Manifest{Name: name}.WithDeployTarget(kTarget), nil
}

func NewK8sOnlyManifestFromYAML(yaml string) (model.Manifest, error) {
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		return model.Manifest{}, errors.Wrap(err, "NewK8sOnlyManifestFromYAML")
	}

	manifest, err := NewK8sOnlyManifest(model.UnresourcedYAMLManifestName, entities)
	if err != nil {
		return model.Manifest{}, errors.Wrap(err, "NewK8sOnlyManifestFromYAML")
	}
	return manifest, nil
}
