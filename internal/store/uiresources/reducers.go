package uiresources

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/store"
)

func HandleUIResourceUpsertAction(state *store.EngineState, action UIResourceUpsertAction) {
	n := action.UIResource.Name
	if state.UIResources[n] != nil && len(state.UIResources[n].Status.DisableStatus.Sources) == 0 && len(action.UIResource.Status.DisableStatus.Sources) != 0 {
		fmt.Printf("adding UIResource %s w/ DisableSource for first time\n", n)
	}
	state.UIResources[n] = action.UIResource
}

func HandleUIResourceDeleteAction(state *store.EngineState, action UIResourceDeleteAction) {
	delete(state.UIResources, action.Name)
}
