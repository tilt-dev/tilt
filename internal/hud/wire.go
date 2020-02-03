package hud

import (
	"os"

	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewRenderer,
	ProvideHud,
	ProvideStdout,
	NewIncrementalPrinter)

func ProvideStdout() Stdout {
	return Stdout(os.Stdout)
}
