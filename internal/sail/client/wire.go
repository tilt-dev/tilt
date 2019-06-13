package client

import (
	"github.com/google/wire"
)

var SailWireSet = wire.NewSet(
	ProvideSailClient,
	wire.Bind(new(SailClient), new(sailClient)),
	ProvideSailRoomer,
	ProvideSailDialer,
)
