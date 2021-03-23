package model

type TriggerMode int

// When Tilt decides that a resource could use a build, TriggerMode specifies whether to require manual approval
// before that build takes place.
// There are two classes of build as far as TriggerMode is concerned:
// 1. Initial - A manifest's first build per `tilt up`. Either directly because the user ran `tilt up`,
// 		or because the user just added the manifest to the Tiltfile.
// 2. Non-initial - After the initial build, any time one of the manifest's dependencies changes, the manifest is ready
//      for an update
// NOTE(maia): These are probably better stored as two different bools (OnFileChange and OnInit
//   or similar)--but that's a refactor for another day
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
