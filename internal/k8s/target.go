package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var pkgInitTime = time.Now()

func MustTarget(name model.TargetName, yaml string) model.K8sTarget {
	entities, err := ParseYAML(strings.NewReader(yaml))
	if err != nil {
		panic(fmt.Errorf("MustTarget: %v", err))
	}
	target, err := NewTarget(name, entities, nil, nil, nil, nil,
		nil, model.PodReadinessIgnore, v1alpha1.KubernetesDiscoveryStrategyDefault,
		nil, nil, model.UpdateSettings{})
	if err != nil {
		panic(fmt.Errorf("MustTarget: %v", err))
	}
	return target
}

func NewTarget(
	name model.TargetName,
	entities []K8sEntity,
	portForwards []model.PortForward,
	extraPodSelectors []labels.Set,
	dependencyIDs []model.TargetID,
	imageTargets []model.ImageTarget,
	refInjectCounts map[string]int,
	podReadinessMode model.PodReadinessMode,
	discoveryStrategy v1alpha1.KubernetesDiscoveryStrategy,
	allLocators []ImageLocator,
	links []model.Link,
	updateSettings model.UpdateSettings) (model.K8sTarget, error) {
	sorted := SortedEntities(entities)
	yaml, err := SerializeSpecYAML(sorted)
	if err != nil {
		return model.K8sTarget{}, err
	}

	objectRefs := make([]v1.ObjectReference, 0, len(sorted))
	for _, e := range sorted {
		objectRefs = append(objectRefs, e.ToObjectReference())
	}

	// Use a min component count of 2 for computing names,
	// so that the resource type appears
	displayNames := UniqueNames(sorted, 2)
	myLocators := []v1alpha1.KubernetesImageLocator{}
	for _, locator := range allLocators {
		if LocatorMatchesOne(locator, entities) {
			myLocators = append(myLocators, locator.ToSpec())
		}
	}

	extraSelectors := SetsAsLabelSelectors(extraPodSelectors)
	sinceTime := apis.NewTime(pkgInitTime)
	apply := v1alpha1.KubernetesApplySpec{
		YAML:          yaml,
		ImageLocators: myLocators,
		Timeout:       metav1.Duration{Duration: updateSettings.K8sUpsertTimeout()},
		KubernetesDiscoveryTemplateSpec: &v1alpha1.KubernetesDiscoveryTemplateSpec{
			ExtraSelectors: extraSelectors,
		},
		PodLogStreamTemplateSpec: &v1alpha1.PodLogStreamTemplateSpec{
			SinceTime: &sinceTime,
			IgnoreContainers: []string{
				string(container.IstioInitContainerName),
				string(container.IstioSidecarContainerName),
			},
		},
		PortForwardTemplateSpec: toPortForwardTemplateSpec(portForwards),
		DiscoveryStrategy:       discoveryStrategy,
	}

	kapp := &v1alpha1.KubernetesApply{Spec: apply}
	errors := kapp.Validate(context.TODO())
	if errors != nil {
		return model.K8sTarget{}, errors.ToAggregate()
	}

	return model.K8sTarget{
		KubernetesApplySpec: apply,
		Name:                name,
		DisplayNames:        displayNames,
		ObjectRefs:          objectRefs,
		PodReadinessMode:    podReadinessMode,
		Links:               links,
	}.WithDependencyIDs(dependencyIDs, model.ToLiveUpdateOnlyMap(imageTargets)).
		WithRefInjectCounts(refInjectCounts), nil
}

func NewK8sOnlyManifest(name model.ManifestName, entities []K8sEntity, allLocators []ImageLocator) (model.Manifest, error) {
	kTarget, err := NewTarget(name.TargetName(), entities, nil, nil, nil, nil,
		nil, model.PodReadinessIgnore, v1alpha1.KubernetesDiscoveryStrategyDefault,
		allLocators, nil, model.UpdateSettings{})
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

	manifest, err := NewK8sOnlyManifest(model.UnresourcedYAMLManifestName, entities, nil)
	if err != nil {
		return model.Manifest{}, errors.Wrap(err, "NewK8sOnlyManifestFromYAML")
	}
	return manifest, nil
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

// Creates a port-forward template if necessary. Returns nil if no port-forwards.
func toPortForwardTemplateSpec(forwards []model.PortForward) *v1alpha1.PortForwardTemplateSpec {
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
