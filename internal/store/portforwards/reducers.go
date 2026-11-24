package portforwards

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandlePortForwardUpsertAction(state *store.EngineState, action PortForwardUpsertAction) {
	obj := action.PortForward
	state.PortForwards[obj.Name] = obj
}

func HandlePortForwardDeleteAction(state *store.EngineState, action PortForwardDeleteAction) {
	delete(state.PortForwards, action.Name)
}
