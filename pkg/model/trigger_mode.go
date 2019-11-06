package model

type TriggerMode int

// How builds are triggered (per manifest or globally):
const (
	// Automatically, whenever we detect a change, or
	TriggerModeAuto TriggerMode = iota
	// Manually (i.e. only when the user tells us to update)
	TriggerModeManualAfterInitial     TriggerMode = iota
	TriggerModeManualIncludingInitial TriggerMode = iota
)

func (t TriggerMode) AutoOnChange() bool {
	return t == TriggerModeAuto
}

func (t TriggerMode) AutoInitial() bool {
	return t != TriggerModeManualIncludingInitial
}
