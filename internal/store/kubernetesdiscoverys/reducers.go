package kubernetesdiscoverys

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func HandleKubernetesDiscoveryUpsertAction(state *store.EngineState, action KubernetesDiscoveryUpsertAction) {
	n := action.KubernetesDiscovery.Name
	state.KubernetesDiscoverys[n] = action.KubernetesDiscovery
	RefreshKubernetesResource(state, n)
	liveupdates.CheckForContainerCrash(state, n)
}

func HandleKubernetesDiscoveryDeleteAction(state *store.EngineState, action KubernetesDiscoveryDeleteAction) {
	delete(state.KubernetesDiscoverys, action.Name)
	RefreshKubernetesResource(state, action.Name)
	liveupdates.CheckForContainerCrash(state, action.Name)
}

func RefreshKubernetesResource(state *store.EngineState, name string) {
	var aStatus *v1alpha1.KubernetesApplyStatus
	a := state.KubernetesApplys[name]
	if a != nil {
		aStatus = &(a.Status)
	}

	d := state.KubernetesDiscoverys[name]
	r, err := k8sconv.NewKubernetesResource(d, aStatus)
	if err != nil {
		return
	}
	state.KubernetesResources[name] = r
}
