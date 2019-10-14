package tiltfile

import (
	"github.com/google/wire"

	"github.com/windmilleng/tilt/internal/tiltfile/k8scontext"
)

var WireSet = wire.NewSet(
	ProvideTiltfileLoader,
	k8scontext.NewExtension,
)
