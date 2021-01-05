package model

// Manifest properties specified by the user in the UI; these override any properties
// specified in the Tiltfile  (but do not persist over multiple runs).
// A `nil` property indicates that it wasn't specified by the user (to differentiate
// from the user specifying the zero value).
type Overrides struct {
	TriggerMode *TriggerMode
}
