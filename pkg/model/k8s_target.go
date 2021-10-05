package model

import (
	"fmt"
	"reflect"
	"strings"

	v1 "k8s.io/api/core/v1"

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

// wait until the pod has completed
const PodReadinessComplete PodReadinessMode = "complete"

type K8sTarget struct {
	// An apiserver-driven data model for applying Kubernetes YAML.
	//
	// This will eventually replace K8sTarget. We represent this as an embedded
	// struct while we're migrating fields.
	v1alpha1.KubernetesApplySpec

	Name         TargetName
	PortForwards []PortForward

	// Each K8s entity should have a display name for user interfaces
	// that balances brevity and uniqueness
	DisplayNames []string

	// Store the name, namespace, and type in a structured form
	// for easy access. This should duplicate what's specified in the YAML.
	ObjectRefs []v1.ObjectReference

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

	depIDs []TargetID
}

func NewK8sTargetForTesting(yaml string) K8sTarget {
	apply := v1alpha1.KubernetesApplySpec{
		YAML: yaml,
	}
	return K8sTarget{KubernetesApplySpec: apply}
}

func (k8s K8sTarget) Empty() bool { return reflect.DeepEqual(k8s, K8sTarget{}) }

func (k8s K8sTarget) HasJob() bool {
	for _, ref := range k8s.ObjectRefs {
		if strings.Contains(ref.Kind, "Job") {
			return true
		}
	}
	return false
}

func (k8s K8sTarget) DependencyIDs() []TargetID {
	return append([]TargetID{}, k8s.depIDs...)
}

func (k8s K8sTarget) RefInjectCounts() map[string]int {
	return k8s.refInjectCounts
}

func (k8s K8sTarget) Validate() error {
	if k8s.ID().Empty() {
		return fmt.Errorf("[Validate] K8s resources missing name:\n%s", k8s.YAML)
	}

	if k8s.YAML == "" {
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

// Track which objects this target depends on inside the manifest.
//
// We're disentangling ImageTarget and live updates -
// image builds may or may not have live updates attached, but
// also live updates may or may not have image builds attached.
//
// KubernetesApplySpec only depends on ImageTargets with an Image build.
//
// The depIDs field depends on ImageTargets that have image builds OR have live
// updates.
//
// ids: a list of the images we directly depend on.
// isLiveUpdateOnly: a map of images that are live-update-only
func (k8s K8sTarget) WithDependencyIDs(ids []TargetID, isLiveUpdateOnly map[TargetID]bool) K8sTarget {
	ids = DedupeTargetIDs(ids)
	k8s.depIDs = ids

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
