package tiltfile

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Simulate two tiltfiles adding and removing services,
// and make sure the order is reasonable.
func TestManifestOrder(t *testing.T) {
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
	state := store.NewState()

	tfMain := model.MainTiltfileManifestName
	tfExtra := model.ManifestName("tf-extra")
	state.TiltfileStates[tfExtra] = &store.ManifestState{
		Name:          tfExtra,
		BuildStatuses: make(map[model.TargetID]*store.BuildStatus),
	}

	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name: tfMain,
		Manifests: []model.Manifest{
			model.Manifest{Name: "a"},
			model.Manifest{Name: "b"},
			model.Manifest{Name: "c"},
		},
	})
	assert.Equal(t,
		[]model.ManifestName{"a", "b", "c"},
		state.ManifestDefinitionOrder)

	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name: tfExtra,
		Manifests: []model.Manifest{
			model.Manifest{Name: "extra-x"},
			model.Manifest{Name: "extra-y"},
			model.Manifest{Name: "extra-z"},
		},
	})
	assert.Equal(t,
		[]model.ManifestName{"a", "b", "c", "extra-x", "extra-y", "extra-z"},
		state.ManifestDefinitionOrder)

	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name: tfMain,
		Manifests: []model.Manifest{
			model.Manifest{Name: "b"},
			model.Manifest{Name: "d"},
		},
	})
	assert.Equal(t,
		[]model.ManifestName{"b", "extra-x", "extra-y", "extra-z", "d"},
		state.ManifestDefinitionOrder)

	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name: tfExtra,
		Manifests: []model.Manifest{
			model.Manifest{Name: "extra-x"},
			model.Manifest{Name: "extra-omega"},
		},
	})
	assert.Equal(t,
		[]model.ManifestName{"b", "extra-x", "d", "extra-omega"},
		state.ManifestDefinitionOrder)

	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name: tfMain,
		Manifests: []model.Manifest{
			model.Manifest{Name: "a"},
			model.Manifest{Name: "b"},
			model.Manifest{Name: "c"},
			model.Manifest{Name: "d"},
		},
	})
	assert.Equal(t,
		[]model.ManifestName{"b", "extra-x", "d", "extra-omega", "a", "c"},
		state.ManifestDefinitionOrder)
}
