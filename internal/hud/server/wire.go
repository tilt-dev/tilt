package server

import (
	"github.com/google/wire"

	"github.com/tilt-dev/tilt/pkg/model"
)

var WireSet = wire.NewSet(
	NewBearerToken,
	ProvideWebListener,
	ProvideAPIServerPort,
	ProvideConfigAccess,
	model.ProvideAPIServerName,
	ProvideKeyCert,
	ProvideMemConn,
	ProvideTiltServerOptions,
	ProvideTiltDynamic,
	ProvideHeadsUpServer,
	ProvideHeadsUpServerController,
)
