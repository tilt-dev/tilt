package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func MustTarget(name model.TargetName, yaml string) model.K8sTarget {
	kt, err := NewTargetForYAML(name, yaml, nil)
	if err != nil {
		panic(fmt.Errorf("MustTarget: %v", err))
	}
	return kt
}

func NewTargetForEntities(name model.TargetName, entities []K8sEntity, locators []ImageLocator) (model.K8sTarget, error) {
	entities = SortedEntities(entities)
	yaml, err := SerializeSpecYAML(entities)
	if err != nil {
		return model.K8sTarget{}, err
	}

	applySpec := v1alpha1.KubernetesApplySpec{
		Cluster:           v1alpha1.ClusterNameDefault,
		DiscoveryStrategy: v1alpha1.KubernetesDiscoveryStrategyDefault,
		YAML:              yaml,
	}

	for _, locator := range locators {
		if LocatorMatchesOne(locator, entities) {
			applySpec.ImageLocators = append(applySpec.ImageLocators, locator.ToSpec())
		}
	}

	target, err := NewTarget(name, applySpec, model.PodReadinessIgnore, nil)
	if err != nil {
		return model.K8sTarget{}, err
	}
	return target, nil
}

func NewTargetForYAML(name model.TargetName, yaml string, locators []ImageLocator) (model.K8sTarget, error) {
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		return model.K8sTarget{}, err
	}
	return NewTargetForEntities(name, entities, locators)
}

func NewTarget(
	name model.TargetName,
	applySpec v1alpha1.KubernetesApplySpec,
	podReadinessMode model.PodReadinessMode,
	links []model.Link) (model.K8sTarget, error) {

	kapp := &v1alpha1.KubernetesApply{Spec: applySpec}
	err := kapp.Validate(context.TODO())
	if err != nil {
		return model.K8sTarget{}, err.ToAggregate()
	}

	return model.K8sTarget{
		KubernetesApplySpec: applySpec,
		Name:                name,
		PodReadinessMode:    podReadinessMode,
		Links:               links,
	}, nil
}

func ParseImageLocators(locators []v1alpha1.KubernetesImageLocator) ([]ImageLocator, error) {
	result := []ImageLocator{}
	for _, locator := range locators {
		selector, err := ParseObjectSelector(locator.ObjectSelector)
		if err != nil {
			return nil, errors.Wrap(err, "parsing image locator")
		}

		if locator.Object != nil {
			parsedLocator, err := NewJSONPathImageObjectLocator(selector, locator.Path, locator.Object.RepoField, locator.Object.TagField)
			if err != nil {
				return nil, errors.Wrap(err, "parsing image locator")
			}
			result = append(result, parsedLocator)
		} else {
			parsedLocator, err := NewJSONPathImageLocator(selector, locator.Path)
			if err != nil {
				return nil, errors.Wrap(err, "parsing image locator")
			}
			result = append(result, parsedLocator)
		}
	}
	return result, nil
}

// PortForwardTemplateSpec creates a port-forward template if necessary. Returns nil if no port-forwards.
func PortForwardTemplateSpec(forwards []model.PortForward) *v1alpha1.PortForwardTemplateSpec {
	if len(forwards) == 0 {
		return nil
	}

	res := make([]v1alpha1.Forward, len(forwards))
	for i, fwd := range forwards {
		res[i] = v1alpha1.Forward{
			LocalPort:     int32(fwd.LocalPort),
			ContainerPort: int32(fwd.ContainerPort),
			Host:          fwd.Host,
			Name:          fwd.Name,
			Path:          fwd.PathForAppend(),
		}
	}
	return &v1alpha1.PortForwardTemplateSpec{
		Forwards: res,
	}
}

func SetsAsLabelSelectors(sets []labels.Set) []metav1.LabelSelector {
	var extraSelectors []metav1.LabelSelector
	for _, s := range sets {
		extraSelectors = append(extraSelectors, *metav1.SetAsLabelSelector(s))
	}
	return extraSelectors
}
