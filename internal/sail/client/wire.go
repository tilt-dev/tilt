package client

import "github.com/google/wire"

var SailWireSet = wire.NewSet(
	ProvideSailClient,
	ProvideSailDialer,
)
