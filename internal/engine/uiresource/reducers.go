package uiresource

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func HandleUIResourceCreateAction(state *store.EngineState, a UIResourceCreateAction) {
	key := apis.Key(a.UIResource)
	if _, ok := state.UIResources[key]; !ok {
		state.UIResources[key] = a.UIResource
	}
}

func HandleUIResourceUpdateStatusAction(state *store.EngineState, a UIResourceUpdateStatusAction) {
	key := apis.KeyFromMeta(*a.ObjectMeta)
	if orig, ok := state.UIResources[key]; ok {
		state.UIResources[key] = &v1alpha1.UIResource{
			ObjectMeta: orig.ObjectMeta,
			Spec:       orig.Spec,
			Status:     *a.Status,
		}
	}
}

func HandleUIResourceDeleteAction(state *store.EngineState, a UIResourceDeleteAction) {
	delete(state.UIResources, a.Name)
}
