package k8swatch

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
)

func HandleKubernetesDiscoveryCreateAction(_ context.Context, state *store.EngineState, a KubernetesDiscoveryCreateAction) {
	key := apis.Key(a.KubernetesDiscovery)
	if _, ok := state.KubernetesDiscoveries[key]; !ok {
		state.KubernetesDiscoveries[key] = a.KubernetesDiscovery
	}
}

func HandleKubernetesDiscoveryUpdateAction(_ context.Context, state *store.EngineState, a KubernetesDiscoveryUpdateAction) {
	key := apis.Key(a.KubernetesDiscovery)
	if _, ok := state.KubernetesDiscoveries[key]; ok {
		state.KubernetesDiscoveries[key] = a.KubernetesDiscovery
	}
}

func HandleKubernetesDiscoveryDeleteAction(_ context.Context, state *store.EngineState, a KubernetesDiscoveryDeleteAction) {
	delete(state.KubernetesDiscoveries, a.Name)
}
