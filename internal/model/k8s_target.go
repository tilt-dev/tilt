package model

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/yaml"
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

	dependencyIDs []TargetID
}

func (k8s K8sTarget) Empty() bool { return reflect.DeepEqual(k8s, K8sTarget{}) }

func (k8s K8sTarget) DependencyIDs() []TargetID {
	return k8s.dependencyIDs
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

func (k8s K8sTarget) AppendYAML(y string) K8sTarget {
	if k8s.YAML == "" {
		k8s.YAML = y
	} else {
		k8s.YAML = yaml.ConcatYAML(k8s.YAML, y)
	}
	return k8s
}

var _ TargetSpec = K8sTarget{}
