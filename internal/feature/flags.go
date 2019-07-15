package feature

import (
	"fmt"
	"sync"
)

const MultipleContainersPerPod = "multiple_containers_per_pod"

type Defaults map[string]bool

// All feature flags need to be defined here with their default values
var flags = Defaults{
	MultipleContainersPerPod: false,
}

type Feature interface {
	IsEnabled(flag string) bool
	Enable(flag string) error
	Disable(flag string) error
	GetAllFlags() map[string]bool
}

func ProvideFeature() Feature {
	return newStaticMapFeature(flags)
}

func ProvideFeatureForTesting(d Defaults) Feature {
	return newStaticMapFeature(d)
}

func newStaticMapFeature(defaults Defaults) *staticMapFeature {
	// copy map so we don't rely on global state
	newMap := map[string]bool{}
	for key, value := range defaults {
		newMap[key] = value
	}
	return &staticMapFeature{flags: newMap, mu: &sync.Mutex{}}
}

type staticMapFeature struct {
	flags map[string]bool
	mu    *sync.Mutex
}

// IsEnabled panics if flag does not exist
func (f *staticMapFeature) IsEnabled(flag string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	enabled, ok := f.flags[flag]
	if !ok {
		panic("Unknown feature flag: " + flag)
	}

	return enabled
}

func (f *staticMapFeature) Enable(flag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.flags[flag]
	if !ok {
		return fmt.Errorf("Unknown feature flag: %s", flag)
	}

	f.flags[flag] = true
	return nil
}

func (f *staticMapFeature) Disable(flag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.flags[flag]
	if !ok {
		return fmt.Errorf("Unknown feature flag: %s", flag)
	}

	f.flags[flag] = false
	return nil
}

// GetAllFlags make a copy of the feature flags map and returns it
func (f *staticMapFeature) GetAllFlags() map[string]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	newFlags := make(map[string]bool, len(f.flags))

	for k, v := range f.flags {
		newFlags[k] = v
	}

	return newFlags
}
