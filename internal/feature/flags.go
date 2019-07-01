package feature

import (
	"fmt"
	"sync"
)

var once sync.Once
var instance Feature

type Defaults map[string]bool

// All feature flags need to be defined here with their default values
var flags = Defaults{
	"events": false,
}

type Feature interface {
	IsEnabled(flag string) (bool, error)
	Enable(flag string) error
	Disable(flag string) error
}

func ProvideFeature() Feature {
	once.Do(func() {
		instance = newStaticMapFeature(flags)
	})

	return instance
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

func (f *staticMapFeature) IsEnabled(flag string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	enabled, ok := f.flags[flag]
	if !ok {
		return false, fmt.Errorf("Unknown flag: %s", flag)
	}

	return enabled, nil
}

func (f *staticMapFeature) Enable(flag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.flags[flag]
	if !ok {
		return fmt.Errorf("Unknown flag: %s", flag)
	}

	f.flags[flag] = true
	return nil
}

func (f *staticMapFeature) Disable(flag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.flags[flag]
	if !ok {
		return fmt.Errorf("Unknown flag: %s", flag)
	}

	f.flags[flag] = false
	return nil
}
