package uiresource

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
)

type UIResourceCreateAction struct {
	UIResource *v1alpha1.UIResource
}

func (p UIResourceCreateAction) Action() {}

func (p UIResourceCreateAction) Summarize(s *store.ChangeSummary) {
	s.UIResources.Add(types.NamespacedName{
		Name:      p.UIResource.Name,
		Namespace: p.UIResource.Namespace,
	})
}

func NewUIResourceCreateAction(kd *v1alpha1.UIResource) UIResourceCreateAction {
	return UIResourceCreateAction{UIResource: kd.DeepCopy()}
}

type UIResourceUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *v1alpha1.UIResourceStatus
}

func (p UIResourceUpdateStatusAction) Action() {}

func (p UIResourceUpdateStatusAction) Summarize(s *store.ChangeSummary) {
	s.UIResources.Add(apis.KeyFromMeta(*p.ObjectMeta))
}

func NewUIResourceUpdateStatusAction(kd *v1alpha1.UIResource) UIResourceUpdateStatusAction {
	return UIResourceUpdateStatusAction{
		ObjectMeta: kd.ObjectMeta.DeepCopy(),
		Status:     kd.Status.DeepCopy(),
	}
}

type UIResourceDeleteAction struct {
	Name types.NamespacedName
}

func (p UIResourceDeleteAction) Action() {}

func (p UIResourceDeleteAction) Summarize(s *store.ChangeSummary) {
	s.UIResources.Add(p.Name)
}

func NewUIResourceDeleteAction(name types.NamespacedName) UIResourceDeleteAction {
	return UIResourceDeleteAction{Name: name}
}
