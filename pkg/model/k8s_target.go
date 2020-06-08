package model

import (
	"fmt"
	"reflect"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type K8sTarget struct {
	Name         TargetName
	YAML         string
	PortForwards []PortForward
	// labels for pods that we should watch and associate with this resource
	ExtraPodSelectors []labels.Selector

	// Each K8s entity should have a display name for user interfaces
	// that balances brevity and uniqueness
	DisplayNames []string

	// Store the name, namespace, and type in a structured form
	// for easy access. This should duplicate what's specified in the YAML.
	ObjectRefs []v1.ObjectReference

	// NonWorkload indicates whether or not a given K8sTarget was
	// determined to have workloads at assembly time during Tiltfile execution
	NonWorkload bool

	dependencyIDs []TargetID

	// Map configRef -> number of times we (expect to) inject it.
	// NOTE(maia): currently this map is only for use in metrics, though someday
	// we want a better way of mapping configRefs -> their injection point(s)
	// (right now, Tiltfile and Engine have two different ways of finding a
	// given image in a k8s entity.
	refInjectCounts map[string]int
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
	return k8s.dependencyIDs
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
	k8s.dependencyIDs = DedupeTargetIDs(ids)
	return k8s
}

func (k8s K8sTarget) WithRefInjectCounts(ric map[string]int) K8sTarget {
	k8s.refInjectCounts = ric
	return k8s
}

var _ TargetSpec = K8sTarget{}
