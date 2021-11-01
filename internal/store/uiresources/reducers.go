package uiresources

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandleUIResourceUpsertAction(state *store.EngineState, action UIResourceUpsertAction) {
	n := action.UIResource.Name
	state.UIResources[n] = action.UIResource
}

func HandleUIResourceDeleteAction(state *store.EngineState, action UIResourceDeleteAction) {
	delete(state.UIResources, action.Name)
}
