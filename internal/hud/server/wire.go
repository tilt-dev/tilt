package server

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	ProvideMemConn,
	ProvideTiltServerOptions,
	ProvideTiltInterface,
	ProvideTiltDynamic,
	ProvideHeadsUpServer,
	ProvideHeadsUpServerController,
)
