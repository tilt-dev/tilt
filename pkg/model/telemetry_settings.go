package model

import "time"

const DefaultTelemetryPeriod = 60 * time.Second

type TelemetrySettings struct {
	Cmd     Cmd
	Workdir string // directory from which this Cmd should be run

	// How often to send the trace data.
	Period time.Duration
}
