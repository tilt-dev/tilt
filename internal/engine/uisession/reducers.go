package uisession

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
)

func HandleUISessionCreateAction(_ context.Context, state *store.EngineState, a UISessionCreateAction) {
	key := apis.Key(a.UISession)
	if _, ok := state.UISessions[key]; !ok {
		state.UISessions[key] = a.UISession
	}
}

func HandleUISessionUpdateStatusAction(ctx context.Context, state *store.EngineState, a UISessionUpdateStatusAction) {
	key := apis.KeyFromMeta(*a.ObjectMeta)
	if _, ok := state.UISessions[key]; ok {
		state.UISessions[key].Status = *a.Status
	}
}
