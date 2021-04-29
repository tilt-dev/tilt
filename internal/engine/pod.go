package engine

import (
	"github.com/tilt-dev/tilt/internal/store"
)

func handlePodResetRestartsAction(state *store.EngineState, action store.PodResetRestartsAction) {
	ms, ok := state.ManifestState(action.ManifestName)
	if !ok {
		return
	}

	runtime := ms.K8sRuntimeState()
	podInfo, ok := runtime.Pods[action.PodID]
	if !ok {
		return
	}

	// We have to be careful here because the pod might have restarted
	// since the action was created.
	delta := store.VisiblePodContainerRestarts(*podInfo) - int32(action.VisibleRestarts)
	podInfo.BaselineRestartCount = store.AllPodContainerRestarts(*podInfo) - delta
}
