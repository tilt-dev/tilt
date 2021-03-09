package model

type TriggerMode int

// When Tilt decides that a resource could use a build, TriggerMode specifies whether to require manual approval
// before that build takes place.
// There are two classes of build as far as TriggerMode is concerned:
// 1. Initial - A manifest's first build per `tilt up`. Either directly because the user ran `tilt up`,
// 		or because the user just added the manifest to the Tiltfile.
// 2. Non-initial - After the initial build, any time one of the manifest's dependencies changes, the manifest is ready
//      for an update
const (
	// Tilt automatically performs initial and non-initial builds without manual intervention
	TriggerModeAuto_AutoInit TriggerMode = iota
	// Tilt automatically performs initial builds without manual intervention, but requires manual intervention for non-initial builds
	TriggerModeManual_AutoInit TriggerMode = iota
	// Tilt requires manual intervention for all builds, and never automatically performs a build
	TriggerModeManual_NoInit TriggerMode = iota
	// Resource does not automatically build on `tilt up`, but builds automatically in response to file changes
	TriggerModeAuto_NoInit TriggerMode = iota
)

var TriggerModes = map[TriggerMode]bool{
	TriggerModeAuto_AutoInit:   true,
	TriggerModeManual_AutoInit: true,
	TriggerModeManual_NoInit:   true,
	TriggerModeAuto_NoInit:     true,
}

func ValidTriggerMode(tm TriggerMode) bool {
	return TriggerModes[tm]
}
func (t TriggerMode) AutoOnChange() bool {
	return t == TriggerModeAuto_AutoInit || t == TriggerModeAuto_NoInit
}

func (t TriggerMode) AutoInitial() bool {
	return t == TriggerModeAuto_AutoInit || t == TriggerModeManual_AutoInit
}
