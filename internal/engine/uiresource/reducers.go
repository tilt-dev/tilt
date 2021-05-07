package uiresource

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
)

func HandleUIResourceCreateAction(_ context.Context, state *store.EngineState, a UIResourceCreateAction) {
	key := apis.Key(a.UIResource)
	if _, ok := state.UIResources[key]; !ok {
		state.UIResources[key] = a.UIResource
	}
}

func HandleUIResourceUpdateStatusAction(ctx context.Context, state *store.EngineState, a UIResourceUpdateStatusAction) {
	key := apis.KeyFromMeta(*a.ObjectMeta)
	if _, ok := state.UIResources[key]; ok {
		state.UIResources[key].Status = *a.Status
	}
}

func HandleUIResourceDeleteAction(_ context.Context, state *store.EngineState, a UIResourceDeleteAction) {
	delete(state.UIResources, a.Name)
}
