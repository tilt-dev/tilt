package k8swatch

import (
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

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

type KubernetesDiscoveryUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *v1alpha1.KubernetesDiscoveryStatus
}

func (p KubernetesDiscoveryUpdateStatusAction) Action() {}

func (p KubernetesDiscoveryUpdateStatusAction) Summarize(s *store.ChangeSummary) {
	s.KubernetesDiscoveries.Add(apis.KeyFromMeta(*p.ObjectMeta))
}

func NewKubernetesDiscoveryUpdateStatusAction(kd *v1alpha1.KubernetesDiscovery) KubernetesDiscoveryUpdateStatusAction {
	return KubernetesDiscoveryUpdateStatusAction{
		ObjectMeta: kd.ObjectMeta.DeepCopy(),
		Status:     kd.Status.DeepCopy(),
	}
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
