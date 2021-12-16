package dcwatch

import (
	"bytes"
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

// How does the `New` function relate to the subscriber we're creating?
func NewDisableSubscriber(dcc dockercompose.DockerComposeClient) *DisableSubscriber {
	return &DisableSubscriber{
		dcc: dcc,
	}
}

func (w *DisableSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	state := st.RLockState()
	project := state.DockerComposeProject()

	if model.IsEmptyDockerComposeProject(project) {
		st.RUnlockState()
		return nil
	}

	var specsToDisable []model.DockerComposeUpSpec
	for _, uir := range state.UIResources {
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
				dcSpec := model.DockerComposeUpSpec{
					Service: string(manifest.State.Name),
					Project: manifest.Manifest.DockerComposeTarget().Spec.Project,
				}
				specsToDisable = append(specsToDisable, dcSpec)
			}
		}
	}

	st.RUnlockState()

	if len(specsToDisable) > 0 {
		// Upon disabling, the DC event watcher will notice the container has stopped and update
		// the resource's RuntimeStatus, preventing it from being re-added to specsToDisable.
		// There is a race here, since this OnChange might get called again before EngineState
		// gets updated with the new RuntimeStatus. This is fine--calling `docker-compose rm` on
		// a down service is a no-op.
		var out bytes.Buffer
		err := w.dcc.Rm(ctx, specsToDisable, &out, &out)
		if err != nil {
			var names []string
			for _, spec := range specsToDisable {
				names = append(names, spec.Service)
			}
			if out.Len() != 0 {
				logger.Get(ctx).Errorf("%s", out.String())
			}
			logger.Get(ctx).Errorf("error stopping disabled docker compose services %v, error: %v", names, err)
		}
	}

	return nil
}
