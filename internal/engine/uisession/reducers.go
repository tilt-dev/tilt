package uisession

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func HandleUISessionCreateAction(state *store.EngineState, a UISessionCreateAction) {
	key := apis.Key(a.UISession)
	if _, ok := state.UISessions[key]; !ok {
		state.UISessions[key] = a.UISession
	}
}

func HandleUISessionUpdateStatusAction(state *store.EngineState, a UISessionUpdateStatusAction) {
	key := apis.KeyFromMeta(*a.ObjectMeta)
	if orig, ok := state.UISessions[key]; ok {
		state.UISessions[key] = &v1alpha1.UISession{
			ObjectMeta: orig.ObjectMeta,
			Spec:       orig.Spec,
			Status:     *a.Status,
		}
	}
}
