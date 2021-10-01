package kubernetesdiscoverys

import (
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func HandleKubernetesDiscoveryUpsertAction(state *store.EngineState, action KubernetesDiscoveryUpsertAction) {
	n := action.KubernetesDiscovery.Name
	oldState := state.KubernetesDiscoverys[n]
	state.KubernetesDiscoverys[n] = action.KubernetesDiscovery

	// We only refresh when the K8sDiscovery is changed.
	//
	// This is really only needed for tests - we have tests that wait until we've
	// reached a steady state, then change some fields on EngineState.
	//
	// K8s controllers assume everything is idempotent, and will wipe out our changes
	// later with duplicate events.
	isChanged := oldState == nil ||
		!apicmp.DeepEqual(oldState.Status, action.KubernetesDiscovery.Status) ||
		!apicmp.DeepEqual(oldState.Spec, action.KubernetesDiscovery.Spec)
	if isChanged {
		RefreshKubernetesResource(state, n)
		liveupdates.CheckForContainerCrash(state, n)
	}
}

func HandleKubernetesDiscoveryDeleteAction(state *store.EngineState, action KubernetesDiscoveryDeleteAction) {
	oldState := state.KubernetesDiscoverys[action.Name]
	delete(state.KubernetesDiscoverys, action.Name)

	isChanged := oldState != nil
	if isChanged {
		RefreshKubernetesResource(state, action.Name)
		liveupdates.CheckForContainerCrash(state, action.Name)
	}
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
