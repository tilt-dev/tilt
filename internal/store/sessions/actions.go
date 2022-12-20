package sessions

import (
	"errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type SessionUpsertAction struct {
	Object *v1alpha1.Session
}

func (SessionUpsertAction) Action() {}

var _ store.Summarizer = SessionUpsertAction{}

func (a SessionUpsertAction) Summarize(summary *store.ChangeSummary) {
	summary.Sessions.Add(types.NamespacedName{Namespace: a.Object.ObjectMeta.Namespace, Name: a.Object.ObjectMeta.Name})
}

func NewSessionUpsertAction(session *v1alpha1.Session) SessionUpsertAction {
	return SessionUpsertAction{Object: session}
}

func HandleSessionUpsertAction(state *store.EngineState, action SessionUpsertAction) {
	status := action.Object.Status
	if status.Done {
		state.ExitSignal = true
		if status.Error != "" {
			state.ExitError = errors.New(status.Error)
		}
	}
}
