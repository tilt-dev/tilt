package feature

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEnabledDefaultsToFalse(t *testing.T) {
	m := newStaticMapFeatureForTesting(map[string]bool{})

	assert.False(t, m.IsEnabled("foo"))
}

func TestIsEnabled(t *testing.T) {
	m := newStaticMapFeatureForTesting(map[string]bool{"foo": true})

	assert.True(t, m.IsEnabled("foo"))
}

func TestEnableUnknownKey(t *testing.T) {
	m := newStaticMapFeatureForTesting(map[string]bool{})
	err := m.Enable("foo")
	assert.EqualError(t, err, "Unknown flag: foo")
}

func TestEnable(t *testing.T) {
	m := newStaticMapFeatureForTesting(map[string]bool{"foo": false})
	err := m.Enable("foo")
	assert.NoError(t, err)
	assert.True(t, m.IsEnabled("foo"))
}

func TestDisableUnknownKey(t *testing.T) {
	m := newStaticMapFeatureForTesting(map[string]bool{})
	err := m.Disable("foo")
	assert.EqualError(t, err, "Unknown flag: foo")
}

func TestDisable(t *testing.T) {
	m := newStaticMapFeatureForTesting(map[string]bool{"foo": true})
	err := m.Disable("foo")
	assert.NoError(t, err)
	assert.False(t, m.IsEnabled("foo"))
}

func newStaticMapFeatureForTesting(flags map[string]bool) *staticMapFeature {
	return &staticMapFeature{flags: flags, mu: &sync.Mutex{}}
}
