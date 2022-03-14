package dockercomposeservices

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandleDockerComposeServiceUpsertAction(state *store.EngineState, action DockerComposeServiceUpsertAction) {
	n := action.DockerComposeService.Name
	state.DockerComposeServices[n] = action.DockerComposeService
}

func HandleDockerComposeServiceDeleteAction(state *store.EngineState, action DockerComposeServiceDeleteAction) {
	delete(state.DockerComposeServices, action.Name)
}
