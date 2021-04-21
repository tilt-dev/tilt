package portforward

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandlePortForwardCreateAction(state *store.EngineState, action PortForwardCreateAction) {
	pf := action.PortForward
	_, exists := state.PortForwards[pf.Name]
	if !exists {
		state.PortForwards[pf.Name] = pf
	}
}

func HandlePortForwardDeleteAction(state *store.EngineState, action PortForwardDeleteAction) {
	delete(state.PortForwards, action.Name)
}
