package model

type TriggerMode int

// Currently TriggerMode models two orthogonal attributes in one enum:
//
// 1. Whether a file change should update the resource immediately (auto vs
//    manual mode)
//
// 2. Whether a resource should start when the env starts (auto_init=true vs
//    auto_init=false mode, sometimes called AutoInit vs ManualInit mode)
//
// In the APIServer, we don't model these as attributes of a resource.  Rather,
// the resource specifies where these attribute comes from, with the ability to
// define different sources.
//
// Update triggers are modeled as StartOn/RestartOn, so resources
// can be configured with different types of update triggers.
//
// Start/stop status of resources are modeled as DisableOn, so
// that individual objects can be independently started/stopped.
//
// We expect TriggerMode to go away in the API in favor of better visualization
// of why updates have been triggered and why resources are stopped.
const (
	// Tilt automatically performs all builds (initial and non-initial) without manual intervention
	TriggerModeAuto TriggerMode = iota
	// Tilt automatically performs initial builds without manual intervention, but requires manual intervention for non-initial builds
	TriggerModeManualWithAutoInit TriggerMode = iota
	// Tilt requires manual intervention for all builds, and never automatically performs a build
	TriggerModeManual TriggerMode = iota
	// Resource does not automatically build on `tilt up`, but builds automatically in response to file changes
	TriggerModeAutoWithManualInit TriggerMode = iota
)

var TriggerModes = map[TriggerMode]bool{
	TriggerModeAuto:               true,
	TriggerModeManualWithAutoInit: true,
	TriggerModeManual:             true,
	TriggerModeAutoWithManualInit: true,
}

func ValidTriggerMode(tm TriggerMode) bool {
	return TriggerModes[tm]
}
func (t TriggerMode) AutoOnChange() bool {
	return t == TriggerModeAuto || t == TriggerModeAutoWithManualInit
}

func (t TriggerMode) AutoInitial() bool {
	return t == TriggerModeAuto || t == TriggerModeManualWithAutoInit
}
