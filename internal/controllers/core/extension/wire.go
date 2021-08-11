package extension

import "github.com/google/wire"

var WireSet = wire.NewSet(
	NewReconciler,
)
