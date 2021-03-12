package controllers

import (
	"github.com/google/wire"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var controllerSet = wire.NewSet(
	filewatch.NewController,

	ProvideControllers,
)

func ProvideControllers(fileWatch *filewatch.Controller, lc *local.Controller) []Controller {
	return []Controller{
		fileWatch,
		lc,
	}
}

var WireSet = wire.NewSet(
	NewTiltServerControllerManager,

	v1alpha1.NewScheme,
	NewControllerBuilder,
	NewClientBuilder,

	ProvideDeferredClient,
	wire.Bind(new(ctrlclient.Client), new(*DeferredClient)),

	controllerSet,
)
