package sidecar

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideSyncletImageRef,
	ProvideSyncletContainer)
