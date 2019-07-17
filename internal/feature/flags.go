package feature

import (
	"fmt"
)

const MultipleContainersPerPod = "multiple_containers_per_pod"
const Events = "events"

type Status int

const (
	Active Status = iota
	Noop
	Warn
	// After Warn is Error, but it's not a value we can set
)

type ObsoleteError string

func (s ObsoleteError) Error() string {
	return string(s)
}

type Value struct {
	Enabled bool
	Status  Status
}

type Defaults map[string]Value

var MainDefaults = Defaults{
	MultipleContainersPerPod: Value{
		Enabled: false,
		Status:  Active,
	},
	Events: Value{
		Enabled: true,
		Status:  Warn,
	},
}

type FeatureSet map[string]Value

func FromDefaults(d Defaults) FeatureSet {
	r := make(FeatureSet)
	for k, v := range d {
		r[k] = v
	}
	return r
}

func (s FeatureSet) Set(name string, enabled bool) error {
	v, ok := s[name]
	if !ok {
		return fmt.Errorf("Unknown feature flag: %s", name)
	}

	switch v.Status {
	case Warn:
		return ObsoleteError(fmt.Sprintf("Obsolete feature flag: %s", name))
	case Noop:
		return nil
	}

	v.Enabled = enabled
	s[name] = v
	return nil
}

func (s FeatureSet) Get(name string) bool {
	return s[name].Enabled
}

func (s FeatureSet) ToEnabled() map[string]bool {
	r := make(map[string]bool)
	for k, v := range s {
		r[k] = v.Enabled
	}
	return r
}
