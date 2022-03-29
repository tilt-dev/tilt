package feature

import (
	"fmt"
)

// The status of a feature flag
// It starts as Active (we're using this flag to evaluate the feature)
// Then when we no longer need it, we make it a noop. Leave it for an upgrade cycle.
// Then move it to Obsolete, which will cause a warning. Leave it for an upgrade cycle.
// Then remove the flag altogether.
type Status int

const (
	Active Status = iota
	Noop
	Obsolete
	// After Obsolete is Error, but it's not a value we can set
)

const MultipleContainersPerPod = "multiple_containers_per_pod"
const Events = "events"
const Snapshots = "snapshots"
const UpdateHistory = "update_history"
const Facets = "facets"
const Labels = "labels"
const LiveUpdateV2 = "live_update_v2"
const DisableResources = "disable_resources"
const BulkDisableResources = "bulk_disable_resources"

// The Value a flag can have. Status should never be changed.
type Value struct {
	Enabled bool
	Status  Status
}

// Defaults is the initial values for a FeatureSet.
// Don't modify after initializing.
type Defaults map[string]Value

// MainDefaults is the defaults we use in Main
var MainDefaults = Defaults{
	MultipleContainersPerPod: Value{
		Enabled: true,
		Status:  Obsolete,
	},
	Events: Value{
		Enabled: true,
		Status:  Obsolete,
	},
	Snapshots: Value{
		Enabled: true,
		// Snapshots FF is used by disable_snapshots() which hides the button
		// in the web UI
		Status: Active,
	},
	UpdateHistory: Value{
		Enabled: false,
		Status:  Obsolete,
	},
	Facets: Value{
		Enabled: true,
		Status:  Obsolete,
	},
	Labels: Value{
		Enabled: true,
		Status:  Obsolete,
	},
	LiveUpdateV2: Value{
		Enabled: true,
		Status:  Obsolete,
	},
	DisableResources: Value{
		Enabled: true,
		Status:  Active,
	},
	BulkDisableResources: Value{
		Enabled: false,
		Status:  Obsolete,
	},
}

// FeatureSet is a mutable set of Features.
type FeatureSet map[string]Value

// Create a FeatureSet from defaults.
func FromDefaults(d Defaults) FeatureSet {
	r := make(FeatureSet)
	for k, v := range d {
		r[k] = v
	}
	return r
}

// ObsoleteError is an error that a feature flag is obsolete
type ObsoleteError string

func (s ObsoleteError) Error() string {
	return string(s)
}

// Set sets enabled for a feature if it's active. Returns an error if flag is unknown or obsolete.
func (s FeatureSet) Set(name string, enabled bool) error {
	v, ok := s[name]
	if !ok {
		return fmt.Errorf("Unknown feature flag: %s", name)
	}

	switch v.Status {
	case Obsolete:
		return ObsoleteError(fmt.Sprintf("Obsolete feature flag: %s", name))
	case Noop:
		return nil
	}

	v.Enabled = enabled
	s[name] = v
	return nil
}

// Get gets whether a feature is enabled.
func (s FeatureSet) Get(name string) bool {
	v, ok := s[name]
	if !ok {
		panic("get of unknown feature flag (code should use feature.Foo instead of \"foo\" to get a flag)")
	}
	return v.Enabled
}

// ToEnabled returns a copy of the enabled values of the FeatureSet
func (s FeatureSet) ToEnabled() map[string]bool {
	r := make(map[string]bool)
	for k, v := range s {
		r[k] = v.Enabled
	}
	return r
}
