package server

import (
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	ProvideTiltServerOptions,
	ProvideTiltInterface,
	ProvideTiltDynamic,
	ProvideHeadsUpServer,
	ProvideHeadsUpServerController,
)
