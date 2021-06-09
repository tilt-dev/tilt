package model

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type K8sImageLocator interface {
	EqualsImageLocator(other interface{}) bool
}

// Whether or not to wait for pods to become ready before
// marking the k8s resource healthy.
//
// TODO(nick): I strongly suspect we will at least want a separate mode
// for jobs that waits until they become complete, as we do in `tilt ci`
type PodReadinessMode string

// Pod readiness isn't applicable to this resource
const PodReadinessNone PodReadinessMode = ""

// Always wait for pods to become ready.
const PodReadinessWait PodReadinessMode = "wait"

// Don't even wait for pods to appear.
const PodReadinessIgnore PodReadinessMode = "ignore"

type K8sTarget struct {
	// An apiserver-driven data model for applying Kubernetes YAML.
	//
	// This will eventually replace K8sTarget. We represent this as an embedded
	// struct while we're migrating fields.
	v1alpha1.KubernetesApplySpec

	Name         TargetName
	PortForwards []PortForward
	// labels for pods that we should watch and associate with this resource
	ExtraPodSelectors []labels.Set

	// Each K8s entity should have a display name for user interfaces
	// that balances brevity and uniqueness
	DisplayNames []string

	// Store the name, namespace, and type in a structured form
	// for easy access. This should duplicate what's specified in the YAML.
	ObjectRefs []v1.ObjectReference

	PodReadinessMode PodReadinessMode

	// Implementations of k8s.ImageLocator
	//
	// NOTE(nick): Untangling the circular dependency between k8s and pkg/model is
	// a longer project. The k8s package needs to be split up a bit between the
	// API objects and the client objects.
	ImageLocators []K8sImageLocator

	// Map configRef -> number of times we (expect to) inject it.
	// NOTE(maia): currently this map is only for use in metrics, though someday
	// we want a better way of mapping configRefs -> their injection point(s)
	// (right now, Tiltfile and Engine have two different ways of finding a
	// given image in a k8s entity.
	refInjectCounts map[string]int

	// zero+ links assoc'd with this resource (to be displayed in UIs,
	// in addition to any port forwards/LB endpoints)
	Links []Link
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
	result := make([]TargetID, 0, len(k8s.ImageMaps))
	for _, name := range k8s.ImageMaps {
		result = append(result, TargetID{
			Type: TargetTypeImage,
			Name: TargetName(name),
		})
	}
	return result
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

func (k8s K8sTarget) WithDependencyIDs(ids []TargetID) K8sTarget {
	ids = DedupeTargetIDs(ids)
	k8s.ImageMaps = make([]string, 0, len(ids))

	for _, id := range ids {
		if id.Type != TargetTypeImage {
			panic(fmt.Sprintf("Invalid k8s dependency: %+v", id))
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
