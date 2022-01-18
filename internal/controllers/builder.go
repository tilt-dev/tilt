package controllers

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Controller interface {
	reconcile.Reconciler
	CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error)
}

// Little helper class to propagate the logger
// from the setup phase.
//
// Discussion:
// https://github.com/kubernetes-sigs/controller-runtime/issues/1752
type ctrlWrapper struct {
	ctx context.Context
	reconcile.Reconciler
}

// Propagate the logger and analytics from setup
func (w ctrlWrapper) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logger.WithLogger(ctx, logger.Get(w.ctx))
	ctx = analytics.WithAnalytics(ctx, analytics.Get(w.ctx))
	return w.Reconciler.Reconcile(ctx, req)
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
var _ store.TearDowner = &ControllerBuilder{}

func (c *ControllerBuilder) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) error {
	return nil
}

func (c *ControllerBuilder) SetUp(ctx context.Context, st store.RStore) error {
	mgr := c.tscm.GetManager()
	client := c.tscm.GetClient()

	if mgr == nil || client == nil {
		return errors.New("controller manager not initialized")
	}

	// create all the builders and THEN start them all - if each builder is created + started,
	// initialization will fail because indexes cannot be added to an Informer after start, and
	// the builders register informers
	builders := make([]*builder.Builder, 0, len(c.controllers))
	for _, controller := range c.controllers {
		b, err := controller.CreateBuilder(mgr)
		if err != nil {
			return fmt.Errorf("error creating builder: %v", err)
		}
		builders = append(builders, b)
	}

	for i, b := range builders {
		wrapper := ctrlWrapper{ctx: ctx, Reconciler: c.controllers[i]}
		if err := b.Complete(wrapper); err != nil {
			return fmt.Errorf("error starting controller: %v", err)
		}
	}

	// start the controller manager now that all the controllers are initialized
	go func() {
		if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			err = fmt.Errorf("controller manager stopped unexpectedly: %v", err)
			st.Dispatch(store.NewErrorAction(err))
		}
	}()

	return nil
}

func (c *ControllerBuilder) TearDown(ctx context.Context) {
	for _, controller := range c.controllers {
		td, ok := controller.(store.TearDowner)
		if ok {
			td.TearDown(ctx)
		}
	}
}
