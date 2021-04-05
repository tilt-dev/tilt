package k8swatch

import (
	"net/url"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type PodChangeAction struct {
	Pod          *v1.Pod
	ManifestName model.ManifestName

	// The UID that we matched against to associate this pod with Tilt.
	// Might be the Pod UID itself, or the UID of an ancestor.
	MatchedAncestorUID types.UID
}

var _ store.Summarizer = PodChangeAction{}

func (PodChangeAction) Action() {}

func (a PodChangeAction) Summarize(s *store.ChangeSummary) {
	s.Pods.Add(types.NamespacedName{Name: string(a.Pod.Name), Namespace: string(a.Pod.Namespace)})
}

func NewPodChangeAction(pod *v1.Pod, mn model.ManifestName, matchedAncestorUID types.UID) PodChangeAction {
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
