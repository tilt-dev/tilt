package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/model"
)

func NewTarget(
	name model.TargetName,
	entities []K8sEntity,
	portForwards []model.PortForward,
	extraPodLabels []labels.Set) (model.K8sTarget, error) {
	yaml, err := SerializeYAML(entities)
	if err != nil {
		return model.K8sTarget{}, err
	}

	var resourceNames []string
	for _, e := range entities {
		resourceNames = append(resourceNames, fmt.Sprintf("%s (%s)", e.Name(), e.Kind.Kind))
	}

	return model.K8sTarget{
		Name:           name,
		YAML:           yaml,
		ResourceNames:  resourceNames,
		PortForwards:   portForwards,
		ExtraPodLabels: extraPodLabels,
	}, nil
}

func NewK8sOnlyManifest(name model.ManifestName, entities []K8sEntity) (model.Manifest, error) {
	kTarget, err := NewTarget(name.TargetName(), entities, nil, nil)
	if err != nil {
		return model.Manifest{}, err
	}
	return model.Manifest{Name: name}.WithDeployTarget(kTarget), nil
}

func NewK8sOnlyManifestForTesting(name model.ManifestName, yaml string) model.Manifest {
	return model.Manifest{Name: name}.
		WithDeployTarget(model.K8sTarget{Name: name.TargetName(), YAML: yaml})
}
