package uiresources

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func HandleUIResourceUpsertAction(state *store.EngineState, action UIResourceUpsertAction) {
	n := action.UIResource.Name
	state.UIResources[n] = action.UIResource
	if action.UIResource != nil && action.UIResource.Status.DisableStatus.DisabledCount > 0 {
		state.RemoveFromTriggerQueue(model.ManifestName(n))
	}
}

func HandleUIResourceDeleteAction(state *store.EngineState, action UIResourceDeleteAction) {
	delete(state.UIResources, action.Name)
}
