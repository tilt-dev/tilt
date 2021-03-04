package controllers

import (
	"context"
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/store"
)

type Controller interface {
	reconcile.Reconciler
	SetClient(client ctrlclient.Client)
	SetupWithManager(mgr ctrl.Manager) error
}

type ControllerBuilder struct {
	tscm        *TiltServerControllerManager
	controllers []Controller
}

func NewControllerBuilder(tscm *TiltServerControllerManager, controllers []Controller) *ControllerBuilder {
	return &ControllerBuilder{
		tscm:        tscm,
		controllers: controllers,
	}
}

var _ store.Subscriber = &ControllerBuilder{}
var _ store.SetUpper = &ControllerBuilder{}

func (c *ControllerBuilder) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) {}

func (c *ControllerBuilder) SetUp(_ context.Context, _ store.RStore) error {
	mgr := c.tscm.GetManager()
	client := c.tscm.GetClient()

	if mgr == nil || client == nil {
		return errors.New("controller manager not initialized")
	}

	for _, controller := range c.controllers {
		controller.SetClient(client)
		if err := controller.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error initializing %T controller: %v", controller, err)
		}
	}
	return nil
}
