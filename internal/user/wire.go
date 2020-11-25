package user

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewFilePrefs,
	wire.Bind(new(PrefsInterface), new(*filePrefs)),
)
