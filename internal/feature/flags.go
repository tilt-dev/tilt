package feature

import (
	"fmt"
	"sync"
)

// All feature flags need to be defined here with their default values
var flags = map[string]bool{
	"events": false,
}

type Checker interface {
	IsEnabled(flag string) bool
}

type Writer interface {
	Enable(flag string) error
	Disable(flag string) error
}

func NewStaticMapFeature() *staticMapFeature {
	return &staticMapFeature{flags: flags, mu: &sync.Mutex{}}
}

type staticMapFeature struct {
	flags map[string]bool
	mu    *sync.Mutex
}

func (f *staticMapFeature) IsEnabled(flag string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.flags[flag]
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
