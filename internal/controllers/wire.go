package controllers

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideRESTConfig,
	NewTiltServerControllerManager,
)
