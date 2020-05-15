package telemetry

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

type TelemetryScriptRanAction struct {
	Status model.TelemetryStatus
}

func (TelemetryScriptRanAction) Action() {}
