package telemetry

import (
	"github.com/windmilleng/tilt/pkg/model"
)

type TelemetryScriptRanAction struct {
	Status model.TelemetryStatus
}

func (TelemetryScriptRanAction) Action() {}
