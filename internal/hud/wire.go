package hud

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewRenderer,
	NewHud)
