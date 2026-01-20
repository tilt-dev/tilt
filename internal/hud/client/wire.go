package client

import (
	"github.com/google/wire"
	"github.com/mattn/go-colorable"
)

var WireSet = wire.NewSet(
	NewLogStreamer,
	ProvideLogPrinter,
	NewLogFilter,
	ProvideStdout,
	NewIncrementalPrinter,
	NewTerminalStream,
)

func ProvideLogPrinter(filter LogFilter, stdout Stdout) LogPrinter {
	if filter.JSONOutput() {
		return NewJSONPrinter(stdout)
	}
	return NewIncrementalPrinter(stdout)
}

func ProvideStdout() Stdout {
	return Stdout(colorable.NewColorableStdout())
}
