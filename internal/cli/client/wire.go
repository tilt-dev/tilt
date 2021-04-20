package client

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideClientConfig,
	NewGetter,
)
