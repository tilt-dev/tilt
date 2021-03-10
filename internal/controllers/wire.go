package controllers

import (
	"github.com/google/wire"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
)

var controllerSet = wire.NewSet(
	filewatch.NewController,

	ProvideControllers,
)

func ProvideControllers(fileWatch *filewatch.Controller) []Controller {
	return []Controller{
		fileWatch,
	}
}

var WireSet = wire.NewSet(
	NewTiltServerControllerManager,

	NewScheme,
	NewControllerBuilder,
	NewClientBuilder,

	ProvideDeferredClient,
	wire.Bind(new(ctrlclient.Client), new(*DeferredClient)),

	controllerSet,
)
