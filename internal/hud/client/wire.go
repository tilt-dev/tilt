package client

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewLogStreamer,
	ProvideLogPrinter,
	NewLogFilter,
	NewIncrementalPrinter,
	NewTerminalStream,
)

func ProvideLogPrinter(filter LogFilter, stdout Stdout) LogPrinter {
	if filter.JSONOutput() {
		return NewJSONPrinter(stdout)
	}
	return NewIncrementalPrinter(stdout)
}
