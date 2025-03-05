package hud

import (
	"github.com/google/wire"
	"github.com/mattn/go-colorable"
)

var WireSet = wire.NewSet(
	NewRenderer,
	NewHud,
	NewLogFilter,
	NewTerminalStream,
	ProvideStdout,
	NewIncrementalPrinter)

func ProvideStdout() Stdout {
	return Stdout(colorable.NewColorableStdout())
}
