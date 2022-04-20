package cmdimages

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandleCmdImageUpsertAction(state *store.EngineState, action CmdImageUpsertAction) {
	obj := action.CmdImage
	n := obj.Name
	state.CmdImages[n] = obj
}

func HandleCmdImageDeleteAction(state *store.EngineState, action CmdImageDeleteAction) {
	delete(state.CmdImages, action.Name)
}
