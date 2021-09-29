package liveupdate

import "github.com/google/wire"

var WireSet = wire.NewSet(
	NewReconciler,
)
