package tiltfile

import (
	"github.com/google/wire"

	"github.com/tilt-dev/tilt/internal/tiltfile/config"
	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
)

var WireSet = wire.NewSet(
	ProvideTiltfileLoader,
	k8scontext.NewPlugin,
	version.NewPlugin,
	config.NewPlugin,
	tiltextension.NewPlugin,
)
