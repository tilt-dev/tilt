package tiltfiles

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func HandleTiltfileUpsertAction(state *store.EngineState, action TiltfileUpsertAction) {
	n := action.Tiltfile.Name
	mn := model.ManifestName(n)
	state.Tiltfiles[n] = action.Tiltfile

	_, ok := state.TiltfileStates[mn]
	if !ok {
		state.TiltfileStates[mn] = &store.ManifestState{
			Name:          mn,
			BuildStatuses: make(map[model.TargetID]*store.BuildStatus),
			Enabled:       true,
		}
	}

	if mn == model.MainTiltfileManifestName {
		state.UserConfigState.Args = action.Tiltfile.Spec.Args
	}

	for _, x := range state.TiltfileDefinitionOrder {
		if x == mn {
			return // already in the order array
		}
	}
	state.TiltfileDefinitionOrder = append(state.TiltfileDefinitionOrder, mn)
}

func HandleTiltfileDeleteAction(state *store.EngineState, action TiltfileDeleteAction) {
	n := action.Name
	mn := model.ManifestName(n)
	delete(state.Tiltfiles, n)
	delete(state.TiltfileStates, mn)

	for i, x := range state.TiltfileDefinitionOrder {
		if x == mn {
			state.TiltfileDefinitionOrder = append(
				state.TiltfileDefinitionOrder[:i],
				state.TiltfileDefinitionOrder[i+1:]...)
			return
		}
	}
}
