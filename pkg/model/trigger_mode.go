package model

type TriggerMode int

// How builds are triggered (per manifest or globally):
const (
	// Automatically, whenever we detect a change, or
	TriggerModeAuto TriggerMode = iota
	// Manually (i.e. only when the user tells us to update)
	TriggerModeManual
)
