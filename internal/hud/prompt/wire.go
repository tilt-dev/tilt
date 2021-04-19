package prompt

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewTerminalPrompt,
	wire.Value(OpenInput(TTYOpen)))
