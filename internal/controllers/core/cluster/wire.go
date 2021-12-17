package cluster

import "github.com/google/wire"

var WireSet = wire.NewSet(
	NewConnectionManager,
	wire.Bind(new(ClientCache), new(*ConnectionManager)),
)
