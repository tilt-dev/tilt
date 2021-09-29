package kubernetesdiscoverys

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type KubernetesDiscoveryUpsertAction struct {
	KubernetesDiscovery *v1alpha1.KubernetesDiscovery
}

func NewKubernetesDiscoveryUpsertAction(obj *v1alpha1.KubernetesDiscovery) KubernetesDiscoveryUpsertAction {
	return KubernetesDiscoveryUpsertAction{KubernetesDiscovery: obj}
}

func (KubernetesDiscoveryUpsertAction) Action() {}

type KubernetesDiscoveryDeleteAction struct {
	Name string
}

func NewKubernetesDiscoveryDeleteAction(n string) KubernetesDiscoveryDeleteAction {
	return KubernetesDiscoveryDeleteAction{Name: n}
}

func (KubernetesDiscoveryDeleteAction) Action() {}
