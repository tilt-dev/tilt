package tiltfile

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

// Editing pod_readiness must take effect on reload. The ManifestTarget's
// K8sRuntimeState caches PodReadinessMode, so a reused target has to pick up
// the new mode: a pod-less resource flipped to 'ignore' should become ready
// instead of staying pending.
func TestConfigsReloadedUpdatesPodReadinessMode(t *testing.T) {
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
	state := store.NewState()

	name := model.ManifestName("restore")
	waitManifest := model.Manifest{Name: name}.WithDeployTarget(model.K8sTarget{
		Name:             model.TargetName(name),
		PodReadinessMode: model.PodReadinessWait,
	})

	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name:      model.MainTiltfileManifestName,
		Manifests: []model.Manifest{waitManifest},
	})

	mt, ok := state.ManifestTargets[name]
	assert.True(t, ok)

	// A successful deploy that produced no pods (e.g. replicas: 0) is pending
	// under the default 'wait' mode.
	krs := mt.State.K8sRuntimeState()
	krs.HasEverDeployedSuccessfully = true
	mt.State.RuntimeState = krs

	assert.Equal(t, model.PodReadinessWait, mt.State.K8sRuntimeState().PodReadinessMode)
	assert.Equal(t, v1alpha1.RuntimeStatusPending,
		mt.State.K8sRuntimeState().RuntimeStatus())

	ignoreManifest := model.Manifest{Name: name}.WithDeployTarget(model.K8sTarget{
		Name:             model.TargetName(name),
		PodReadinessMode: model.PodReadinessIgnore,
	})
	HandleConfigsReloaded(ctx, state, ConfigsReloadedAction{
		Name:      model.MainTiltfileManifestName,
		Manifests: []model.Manifest{ignoreManifest},
	})

	mt, ok = state.ManifestTargets[name]
	assert.True(t, ok)
	assert.Equal(t, model.PodReadinessIgnore, mt.State.K8sRuntimeState().PodReadinessMode)
	assert.Equal(t, v1alpha1.RuntimeStatusOK,
		mt.State.K8sRuntimeState().RuntimeStatus())
}
