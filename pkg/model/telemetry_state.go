package model

import (
	"time"
)

type TelemetryStatus struct {
	LastRunAt             time.Time
	ControllerActionCount int
}
