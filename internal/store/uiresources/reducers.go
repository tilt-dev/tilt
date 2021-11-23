package uiresources

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func HandleUIResourceUpsertAction(state *store.EngineState, action UIResourceUpsertAction) {
	n := action.UIResource.Name
	old := state.UIResources[n]
	uir := action.UIResource
	if old != nil {
		oldCount := old.Status.DisableStatus.DisabledCount
		newCount := uir.Status.DisableStatus.DisabledCount

		message := ""
		if oldCount == 0 && newCount > 0 {
			message = fmt.Sprintf("Resource %q disabled.\n", n)
		} else if oldCount > 0 && newCount == 0 {
			message = fmt.Sprintf("Resource %q enabled.\n", n)
		}

		if message != "" {
			a := store.NewLogAction(model.ManifestName(n), logstore.SpanID(fmt.Sprintf("disabletoggle-%s", n)), logger.InfoLvl, nil, []byte(message))
			state.LogStore.Append(a, state.Secrets)
		}
	}

	state.UIResources[n] = uir

	if uir != nil && uir.Status.DisableStatus.DisabledCount > 0 {
		state.RemoveFromTriggerQueue(model.ManifestName(n))
	}
}

func HandleUIResourceDeleteAction(state *store.EngineState, action UIResourceDeleteAction) {
	delete(state.UIResources, action.Name)
}
