package k8swatch

import (
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/pkg/model"
)

type KubernetesDiscoveryUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *v1alpha1.KubernetesDiscoveryStatus
}

func (p KubernetesDiscoveryUpdateStatusAction) Action() {}

func NewKubernetesDiscoveryUpdateStatusAction(kd *v1alpha1.KubernetesDiscovery) KubernetesDiscoveryUpdateStatusAction {
	return KubernetesDiscoveryUpdateStatusAction{
		ObjectMeta: kd.ObjectMeta.DeepCopy(),
		Status:     kd.Status.DeepCopy(),
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
