package portforward

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandlePortForwardCreateAction(state *store.EngineState, action PortForwardCreateAction) {
	// will also overwrite an existing PortForward of the same name
	pf := action.PortForward
	state.PortForwards[pf.Name] = pf
}

func HandlePortForwardDeleteAction(state *store.EngineState, action PortForwardDeleteAction) {
	delete(state.PortForwards, action.Name)
}
