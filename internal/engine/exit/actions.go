package exit

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	tiltruns "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type TiltRunUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *tiltruns.TiltRunStatus
}

var _ store.Summarizer = TiltRunUpdateStatusAction{}

func (a TiltRunUpdateStatusAction) Summarize(summary *store.ChangeSummary) {
	summary.TiltRuns.Add(types.NamespacedName{Namespace: a.ObjectMeta.Namespace, Name: a.ObjectMeta.Name})
}

func NewTiltRunUpdateStatusAction(tiltRun *tiltruns.TiltRun) TiltRunUpdateStatusAction {
	return TiltRunUpdateStatusAction{
		ObjectMeta: tiltRun.ObjectMeta.DeepCopy(),
		Status:     tiltRun.Status.DeepCopy(),
	}
}

func (TiltRunUpdateStatusAction) Action() {}

func HandleTiltRunUpdateStatusAction(state *store.EngineState, action TiltRunUpdateStatusAction) {
	if action.Status.Done {
		state.ExitSignal = true
		if action.Status.Error != "" {
			state.ExitError = errors.New(action.Status.Error)
		}
	}
}
