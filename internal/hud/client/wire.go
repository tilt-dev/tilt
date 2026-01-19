package client

import (
	"github.com/google/wire"

	"github.com/tilt-dev/tilt/internal/hud"
)

var WireSet = wire.NewSet(
	NewLogStreamer,
	ProvideLogPrinter,
)

func ProvideLogPrinter(filter hud.LogFilter, stdout hud.Stdout) hud.LogPrinter {
	if filter.JSONOutput() {
		return hud.NewJSONPrinter(stdout)
	}
	return hud.NewIncrementalPrinter(stdout)
}
