package session

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	session "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type SessionUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *session.SessionStatus
}

var _ store.Summarizer = SessionUpdateStatusAction{}

func (a SessionUpdateStatusAction) Summarize(summary *store.ChangeSummary) {
	summary.Sessions.Add(types.NamespacedName{Namespace: a.ObjectMeta.Namespace, Name: a.ObjectMeta.Name})
}

func NewSessionUpdateStatusAction(session *session.Session) SessionUpdateStatusAction {
	return SessionUpdateStatusAction{
		ObjectMeta: session.ObjectMeta.DeepCopy(),
		Status:     session.Status.DeepCopy(),
	}
}

func (SessionUpdateStatusAction) Action() {}

func HandleSessionUpdateStatusAction(state *store.EngineState, action SessionUpdateStatusAction) {
	if action.Status.Done {
		state.ExitSignal = true
		if action.Status.Error != "" {
			state.ExitError = errors.New(action.Status.Error)
		}
	}
}
