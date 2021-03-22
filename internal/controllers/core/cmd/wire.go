package cmd

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideExecer,
	ProvideProberManager,
	NewController,
)
