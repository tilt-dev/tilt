package configs

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/controllers/apis/uibutton"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ConfigsController struct {
	ctrlClient               ctrlclient.Client
	isInitialTiltfileCreated bool
}

func NewConfigsController(ctrlClient ctrlclient.Client) *ConfigsController {
	return &ConfigsController{
		ctrlClient: ctrlClient,
	}
}

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	if !cc.isInitialTiltfileCreated {
		err := cc.maybeCreateInitialTiltfile(ctx, st)
		if err != nil {
			return err
		}
	}

	return nil
}

// Register the tiltfile with the APIServer, then dispatch an action to also copy it into the EngineState.
func (cc *ConfigsController) maybeCreateInitialTiltfile(ctx context.Context, st store.RStore) error {
	state := st.RLockState()
	desired := state.DesiredTiltfilePath
	ucs := state.UserConfigState
	st.RUnlockState()

	err := cc.ctrlClient.Create(ctx, tiltfile.MainTiltfile(desired, ucs.Args))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	err = cc.ctrlClient.Create(ctx, uibutton.StopBuildButton(model.MainTiltfileManifestName.String()))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	cc.isInitialTiltfileCreated = true
	return nil
}
