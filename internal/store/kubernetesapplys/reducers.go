package kubernetesapplys

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func HandleKubernetesApplyUpsertAction(state *store.EngineState, action KubernetesApplyUpsertAction) {
	n := action.KubernetesApply.Name
	state.KubernetesApplys[n] = action.KubernetesApply
}

func HandleKubernetesApplyDeleteAction(state *store.EngineState, action KubernetesApplyDeleteAction) {
	delete(state.KubernetesApplys, action.Name)
}
