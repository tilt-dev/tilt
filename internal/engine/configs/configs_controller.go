package configs

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

	name := model.MainTiltfileManifestName.String()
	fwName := apis.SanitizeName(fmt.Sprintf("%s:%s", model.TargetTypeConfigs, name))
	newTF := &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.TiltfileSpec{
			Path: desired,
			Args: ucs.Args,
			RestartOn: &v1alpha1.RestartOnSpec{
				FileWatches: []string{fwName},
			},
		},
	}
	err := cc.ctrlClient.Create(ctx, newTF)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	cc.isInitialTiltfileCreated = true
	return nil
}
