package controllers

import (
	"github.com/google/wire"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
)

var controllerSet = wire.NewSet(
	filewatch.WireSet,

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

	controllerSet,
)
