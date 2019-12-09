package telemetry

import "time"

type TelemetryScriptRanAction struct {
	At time.Time
}

func (TelemetryScriptRanAction) Action() {}
