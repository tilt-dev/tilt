package controllers

import (
	"github.com/google/wire"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var controllerSet = wire.NewSet(
	filewatch.NewController,

	ProvideControllers,
)

func ProvideControllers(
	fileWatch *filewatch.Controller,
	cmds *cmd.Controller,
	podlogstreams *runtimelog.PodLogStreamController) []Controller {
	return []Controller{
		fileWatch,
		cmds,
		podlogstreams,
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
