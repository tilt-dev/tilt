package runtimelog

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandlePodLogStreamCreateAction(state *store.EngineState, action PodLogStreamCreateAction) {
	pls := action.PodLogStream
	_, exists := state.PodLogStreams[pls.Name]
	if !exists {
		state.PodLogStreams[pls.Name] = pls
	}
}

func HandlePodLogStreamDeleteAction(state *store.EngineState, action PodLogStreamDeleteAction) {
	delete(state.PodLogStreams, action.Name)
}
