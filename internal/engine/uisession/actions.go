package uisession

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
)

type UISessionCreateAction struct {
	UISession *v1alpha1.UISession
}

func (p UISessionCreateAction) Action() {}

func (p UISessionCreateAction) Summarize(s *store.ChangeSummary) {
	s.UISessions.Add(types.NamespacedName{
		Name:      p.UISession.Name,
		Namespace: p.UISession.Namespace,
	})
}

func NewUISessionCreateAction(kd *v1alpha1.UISession) UISessionCreateAction {
	return UISessionCreateAction{UISession: kd.DeepCopy()}
}

type UISessionUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *v1alpha1.UISessionStatus
}

func (p UISessionUpdateStatusAction) Action() {}

func (p UISessionUpdateStatusAction) Summarize(s *store.ChangeSummary) {
	s.UISessions.Add(apis.KeyFromMeta(*p.ObjectMeta))
}

func NewUISessionUpdateStatusAction(kd *v1alpha1.UISession) UISessionUpdateStatusAction {
	return UISessionUpdateStatusAction{
		ObjectMeta: kd.ObjectMeta.DeepCopy(),
		Status:     kd.Status.DeepCopy(),
	}
}
