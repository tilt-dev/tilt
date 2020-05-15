package tiltfile

import (
	"github.com/google/wire"

	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
)

var WireSet = wire.NewSet(
	ProvideTiltfileLoader,
	k8scontext.NewExtension,
	version.NewExtension,
)
