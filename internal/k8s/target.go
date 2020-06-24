package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func MustTarget(name model.TargetName, yaml string) model.K8sTarget {
	entities, err := ParseYAML(strings.NewReader(yaml))
	if err != nil {
		panic(fmt.Errorf("MustTarget: %v", err))
	}
	target, err := NewTarget(context.TODO(), name, entities, nil, nil, nil, nil, false)
	if err != nil {
		panic(fmt.Errorf("MustTarget: %v", err))
	}
	return target
}

func NewTarget(
	ctx context.Context,
	name model.TargetName,
	entities []K8sEntity,
	portForwards []model.PortForward,
	extraPodSelectors []labels.Selector,
	dependencyIDs []model.TargetID,
	refInjectCounts map[string]int,
	nonWorkload bool) (model.K8sTarget, error) {

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
	duplicates := make(map[string]int)

	for _, name := range displayNames {
		duplicates[name[0:len(name)-2]]++
	}

	l := logger.NewLogger(logger.WarnLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	for k, v := range duplicates {
		if v > 1 {
			logger.Get(ctx).Warnf("WARNING: Resource %s has been duplicated", k)
		}
	}
	return model.K8sTarget{
		Name:              name,
		YAML:              yaml,
		PortForwards:      portForwards,
		ExtraPodSelectors: extraPodSelectors,
		DisplayNames:      displayNames,
		ObjectRefs:        objectRefs,
		NonWorkload:       nonWorkload,
	}.WithDependencyIDs(dependencyIDs).WithRefInjectCounts(refInjectCounts), nil
}

func NewK8sOnlyManifest(name model.ManifestName, entities []K8sEntity) (model.Manifest, error) {
	kTarget, err := NewTarget(context.TODO(), name.TargetName(), entities, nil, nil, nil, nil, true)
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
