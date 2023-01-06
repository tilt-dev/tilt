package sessions

import (
	"errors"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type SessionStatusUpdateAction struct {
	Object *v1alpha1.Session
}

func (SessionStatusUpdateAction) Action() {}

func NewSessionStatusUpdateAction(session *v1alpha1.Session) SessionStatusUpdateAction {
	return SessionStatusUpdateAction{Object: session}
}

func HandleSessionStatusUpdateAction(state *store.EngineState, action SessionStatusUpdateAction) {
	status := action.Object.Status
	if status.Done {
		state.ExitSignal = true
		if status.Error != "" {
			state.ExitError = errors.New(status.Error)
		}
	}
}
