package portforward

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandlePortForwardUpsertAction(state *store.EngineState, action PortForwardUpsertAction) {
	// insert, or overwrite an existing PortForward of the same name
	pf := action.PortForward
	state.PortForwards[pf.Name] = pf
}

func HandlePortForwardDeleteAction(state *store.EngineState, action PortForwardDeleteAction) {
	delete(state.PortForwards, action.Name)
}
