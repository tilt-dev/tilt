package kubernetesdiscoverys

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandleKubernetesDiscoveryUpsertAction(state *store.EngineState, action KubernetesDiscoveryUpsertAction) {
	n := action.KubernetesDiscovery.Name
	state.KubernetesDiscoverys[n] = action.KubernetesDiscovery
}

func HandleKubernetesDiscoveryDeleteAction(state *store.EngineState, action KubernetesDiscoveryDeleteAction) {
	delete(state.KubernetesDiscoverys, action.Name)
}
