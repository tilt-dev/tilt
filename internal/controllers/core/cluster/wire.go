package cluster

import (
	"github.com/google/wire"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
)

var WireSet = wire.NewSet(
	NewConnectionManager,
	wire.Bind(new(cluster.ClientProvider), new(*ConnectionManager)),
)
