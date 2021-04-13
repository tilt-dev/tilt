package server

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	NewBearerToken,
	ProvideKeyCert,
	ProvideMemConn,
	ProvideTiltServerOptions,
	ProvideTiltDynamic,
	ProvideHeadsUpServer,
	ProvideHeadsUpServerController,
)
