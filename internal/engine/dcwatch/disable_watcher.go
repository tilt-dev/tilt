package dcwatch

import (
	"context"

	"github.com/tilt-dev/tilt/pkg/logger"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type DisableSubscriber struct {
	dcc dockercompose.DockerComposeClient
}

func NewDisableSubscriber(dcc dockercompose.DockerComposeClient) *DisableSubscriber {
	return &DisableSubscriber{
		dcc: dcc,
	}
}

func (w *DisableSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	if len(summary.UIResources.Changes) == 0 {
		return nil
	}

	state := st.RLockState()
	project := state.DockerComposeProject()

	if model.IsEmptyDockerComposeProject(project) {
		st.RUnlockState()
		return nil
	}

	var uirToDisable *v1alpha1.UIResource
	var specToDisable model.DockerComposeUpSpec
	for nn := range summary.UIResources.Changes {
		uir, ok := state.UIResources[nn.Name]
		if !ok {
			continue
		}
		if uir.Status.DisableStatus.DisabledCount > 0 {
			manifest, exists := state.ManifestTargets[model.ManifestName(uir.Name)]
			if !exists {
				continue
			}

			if !manifest.State.IsDC() {
				continue
			}

			rs := manifest.State.DCRuntimeState().RuntimeStatus()
			if rs == v1alpha1.RuntimeStatusOK || rs == v1alpha1.RuntimeStatusPending {
				// for now, only disable one at a time
				// https://app.shortcut.com/windmill/story/13140/support-logging-to-multiple-manifests
				specToDisable = manifest.Manifest.DockerComposeTarget().Spec
				uirToDisable = uir
				break
			}
		}
	}

	st.RUnlockState()

	if uirToDisable != nil {
		// Upon disabling, the DC event watcher will notice the container has stopped and update
		// the resource's RuntimeStatus, preventing it from being re-added to specsToDisable.
		// There is a race here, since this OnChange might get called again before EngineState
		// gets updated with the new RuntimeStatus. This is fine--calling `docker-compose rm` on
		// a down service is a no-op.

		ctx, err := store.WithObjectLogHandler(ctx, st, uirToDisable)
		if err != nil {
			logger.Get(ctx).Errorf("error creating logger for %s: %v", uirToDisable.Name, err)
			// continue anyway:
			// probably better to try stopping the service instead of re-logging error infinitely
		}
		out := logger.Get(ctx).Writer(logger.InfoLvl)

		err = w.dcc.Rm(ctx, []model.DockerComposeUpSpec{specToDisable}, out, out)
		if err != nil {
			logger.Get(ctx).Errorf("error stopping disabled docker compose service %s, error: %v", specToDisable.Service, err)
		}
	}

	return nil
}
