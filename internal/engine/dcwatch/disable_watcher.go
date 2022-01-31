package dcwatch

import (
	"context"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type DisableSubscriber struct {
	disabler disabler
}

func NewDisableSubscriber(dcc dockercompose.DockerComposeClient, clock clockwork.Clock) *DisableSubscriber {
	return &DisableSubscriber{
		disabler: newDisabler(dcc, clock),
	}
}

func (w *DisableSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	state := st.RLockState()
	defer st.RUnlockState()

	project := state.DockerComposeProject()
	if model.IsEmptyDockerComposeProject(project) {
		return nil
	}

	for i, mn := range state.ManifestDefinitionOrder {
		mt := state.ManifestTargets[mn]
		ms := mt.State

		if !ms.IsDC() {
			continue
		}

		rs := ms.DCRuntimeState().RuntimeStatus()

		isRunning := rs == v1alpha1.RuntimeStatusOK || rs == v1alpha1.RuntimeStatusPending
		isDisabled := ms.DisableState == v1alpha1.DisableStateDisabled
		needsCleanup := isRunning && isDisabled

		w.disabler.Update(ctx, disableQueueEntry{
			Spec:         mt.Manifest.DockerComposeTarget().Spec,
			NeedsCleanup: needsCleanup,
			StartTime:    ms.DCRuntimeState().StartTime,
			Order:        i,
		})
	}

	return nil
}
