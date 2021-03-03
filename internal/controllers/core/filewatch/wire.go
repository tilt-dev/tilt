package filewatch

import "github.com/google/wire"

var WireSet = wire.NewSet(
	NewController,
)
