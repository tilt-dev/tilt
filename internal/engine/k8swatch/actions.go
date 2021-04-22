package k8swatch

import (
	"net/url"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type KubernetesDiscoveryCreateAction struct {
	KubernetesDiscovery *v1alpha1.KubernetesDiscovery
}

func (p KubernetesDiscoveryCreateAction) Action() {}

func (p KubernetesDiscoveryCreateAction) Summarize(s *store.ChangeSummary) {
	s.KubernetesDiscoveries.Add(types.NamespacedName{
		Name:      p.KubernetesDiscovery.Name,
		Namespace: p.KubernetesDiscovery.Namespace,
	})
}

func NewKubernetesDiscoveryCreateAction(kd *v1alpha1.KubernetesDiscovery) KubernetesDiscoveryCreateAction {
	return KubernetesDiscoveryCreateAction{KubernetesDiscovery: kd.DeepCopy()}
}

type KubernetesDiscoveryUpdateAction struct {
	KubernetesDiscovery *v1alpha1.KubernetesDiscovery
}

func (p KubernetesDiscoveryUpdateAction) Action() {}

func (p KubernetesDiscoveryUpdateAction) Summarize(s *store.ChangeSummary) {
	s.KubernetesDiscoveries.Add(types.NamespacedName{
		Name:      p.KubernetesDiscovery.Name,
		Namespace: p.KubernetesDiscovery.Namespace,
	})
}

func NewKubernetesDiscoveryUpdateAction(kd *v1alpha1.KubernetesDiscovery) KubernetesDiscoveryUpdateAction {
	return KubernetesDiscoveryUpdateAction{KubernetesDiscovery: kd.DeepCopy()}
}

type KubernetesDiscoveryDeleteAction struct {
	Name types.NamespacedName
}

func (p KubernetesDiscoveryDeleteAction) Action() {}

func (p KubernetesDiscoveryDeleteAction) Summarize(s *store.ChangeSummary) {
	s.KubernetesDiscoveries.Add(p.Name)
}

func NewKubernetesDiscoveryDeleteAction(name types.NamespacedName) KubernetesDiscoveryDeleteAction {
	return KubernetesDiscoveryDeleteAction{Name: name}
}

type PodChangeAction struct {
	Pod          *v1alpha1.Pod
	ManifestName model.ManifestName

	// The UID that we matched against to associate this pod with Tilt.
	// Might be the Pod UID itself, or the UID of an ancestor.
	MatchedAncestorUID types.UID
}

var _ store.Summarizer = PodChangeAction{}

func (PodChangeAction) Action() {}

func (a PodChangeAction) Summarize(s *store.ChangeSummary) {
	s.Pods.Add(types.NamespacedName{Name: a.Pod.Name, Namespace: a.Pod.Namespace})
}

func NewPodChangeAction(pod *v1alpha1.Pod, mn model.ManifestName, matchedAncestorUID types.UID) PodChangeAction {
	return PodChangeAction{
		Pod:                pod,
		ManifestName:       mn,
		MatchedAncestorUID: matchedAncestorUID,
	}
}

type PodDeleteAction struct {
	PodID     k8s.PodID
	Namespace k8s.Namespace
}

var _ store.Summarizer = PodDeleteAction{}

func (PodDeleteAction) Action() {}

func (a PodDeleteAction) Summarize(s *store.ChangeSummary) {
	s.Pods.Add(types.NamespacedName{Name: string(a.PodID), Namespace: string(a.Namespace)})
}

func NewPodDeleteAction(podID k8s.PodID, namespace k8s.Namespace) PodDeleteAction {
	return PodDeleteAction{
		PodID:     podID,
		Namespace: namespace,
	}
}

type ServiceChangeAction struct {
	Service      *v1.Service
	ManifestName model.ManifestName
	URL          *url.URL
}

func (ServiceChangeAction) Action() {}

func NewServiceChangeAction(service *v1.Service, mn model.ManifestName, url *url.URL) ServiceChangeAction {
	return ServiceChangeAction{
		Service:      service,
		ManifestName: mn,
		URL:          url,
	}
}
