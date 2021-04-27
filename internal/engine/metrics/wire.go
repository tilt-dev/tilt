package metrics

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewController,
)
