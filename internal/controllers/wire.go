package controllers

import "github.com/google/wire"

var WireSet = wire.NewSet(
	NewTiltServerControllerManager,
)
