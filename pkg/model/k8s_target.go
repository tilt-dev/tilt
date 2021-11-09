package model

import (
	"fmt"
	"reflect"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Specifies how a Pod's state factors into determining whether a resource is ready
type PodReadinessMode string

// Pod readiness isn't applicable to this resource
const PodReadinessNone PodReadinessMode = ""

// Always wait for pods to become ready.
const PodReadinessWait PodReadinessMode = "wait"

// Don't even wait for pods to appear.
const PodReadinessIgnore PodReadinessMode = "ignore"

// wait until the pod has completed successfully
const PodReadinessSucceeded PodReadinessMode = "succeeded"

type K8sTarget struct {
	// An apiserver-driven data model for applying Kubernetes YAML.
	//
	// This will eventually replace K8sTarget. We represent this as an embedded
	// struct while we're migrating fields.
	v1alpha1.KubernetesApplySpec

	Name TargetName

	PodReadinessMode PodReadinessMode

	// Map configRef -> number of times we (expect to) inject it.
	// NOTE(maia): currently this map is only for use in metrics, though someday
	// we want a better way of mapping configRefs -> their injection point(s)
	// (right now, Tiltfile and Engine have two different ways of finding a
	// given image in a k8s entity.
	refInjectCounts map[string]int

	// zero+ links assoc'd with this resource (to be displayed in UIs,
	// in addition to any port forwards/LB endpoints)
	Links []Link

	imageDeps []TargetID

	// pathDependencies are files required by this target.
	//
	// For Tiltfile-based, YAML-driven (i.e. `k8s_yaml()`) resources, this is
	// NOT used because it's not sufficient to reload the YAML and re-deploy;
	// there is a lot of post-Tiltfile-load logic for resource assembly, image
	// locator injection, etc. As a result, these resources have their YAML
	// files registered as "config files", which cause the Tiltfile to be
	// re-evaluated.
	pathDependencies []string
	localRepos       []LocalGitRepo
}

func NewK8sTargetForTesting(yaml string) K8sTarget {
	apply := v1alpha1.KubernetesApplySpec{
		YAML: yaml,
	}
	return K8sTarget{KubernetesApplySpec: apply}
}

func (k8s K8sTarget) Empty() bool { return reflect.DeepEqual(k8s, K8sTarget{}) }

func (k8s K8sTarget) DependencyIDs() []TargetID {
	return append([]TargetID{}, k8s.imageDeps...)
}

func (k8s K8sTarget) RefInjectCounts() map[string]int {
	return k8s.refInjectCounts
}

func (k8s K8sTarget) Validate() error {
	if k8s.ID().Empty() {
		return fmt.Errorf("[Validate] K8s resources missing name:\n%s", k8s.YAML)
	}

	// TODO(milas): improve error message
	if k8s.KubernetesApplySpec.YAML == "" && k8s.KubernetesApplySpec.DeployCmd == nil {
		return fmt.Errorf("[Validate] K8s resources %q missing YAML", k8s.Name)
	}

	return nil
}

func (k8s K8sTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeK8s,
		Name: k8s.Name,
	}
}

// LocalRepos is part of the WatchableTarget interface.
func (k8s K8sTarget) LocalRepos() []LocalGitRepo {
	return k8s.localRepos
}

// Dockerignores is part of the WatchableTarget interface.
func (k8s K8sTarget) Dockerignores() []Dockerignore {
	return nil
}

// IgnoredLocalDirectories is part of the WatchableTarget interface.
func (k8s K8sTarget) IgnoredLocalDirectories() []string {
	return nil
}

// Dependencies are files required by this target.
//
// Part of the WatchableTarget interface.
func (k8s K8sTarget) Dependencies() []string {
	// sorting/de-duping guaranteed by setter
	return k8s.pathDependencies
}

// Track which objects this target depends on inside the manifest.
//
// We're disentangling ImageTarget and live updates -
// image builds may or may not have live updates attached, but
// also live updates may or may not have image builds attached.
//
// KubernetesApplySpec only depends on ImageTargets with an Image build.
//
// The imageDeps field depends on ImageTargets that have image builds OR have live
// updates.
//
// ids: a list of the images we directly depend on.
// isLiveUpdateOnly: a map of images that are live-update-only
func (k8s K8sTarget) WithImageDependencies(ids []TargetID, isLiveUpdateOnly map[TargetID]bool) K8sTarget {
	ids = DedupeTargetIDs(ids)
	k8s.imageDeps = ids

	k8s.ImageMaps = make([]string, 0, len(ids))

	for _, id := range ids {
		if id.Type != TargetTypeImage {
			panic(fmt.Sprintf("Invalid k8s dependency: %+v", id))
		}
		if isLiveUpdateOnly[id] {
			continue
		}
		k8s.ImageMaps = append(k8s.ImageMaps, string(id.Name))
	}

	return k8s
}

// WithPathDependencies registers paths that this K8sTarget depends on.
func (k8s K8sTarget) WithPathDependencies(paths []string, localRepos []LocalGitRepo) K8sTarget {
	k8s.pathDependencies = sliceutils.DedupedAndSorted(paths)
	k8s.localRepos = localRepos
	return k8s
}

func (k8s K8sTarget) WithRefInjectCounts(ric map[string]int) K8sTarget {
	k8s.refInjectCounts = ric
	return k8s
}

var _ TargetSpec = K8sTarget{}

func ToLiveUpdateOnlyMap(imageTargets []ImageTarget) map[TargetID]bool {
	result := make(map[TargetID]bool, len(imageTargets))
	for _, image := range imageTargets {
		result[image.ID()] = image.IsLiveUpdateOnly
	}
	return result
}
