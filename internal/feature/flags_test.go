package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEnabledReturnsAnErrorIfKeyDoesntExist(t *testing.T) {
	m := newStaticMapFeature(map[string]bool{})
	assert.Panics(t, func() {
		m.IsEnabled("foo")
	})
}

func TestIsEnabled(t *testing.T) {
	m := newStaticMapFeature(map[string]bool{"foo": true})
	enabled := m.IsEnabled("foo")

	assert.True(t, enabled)
}

func TestEnableUnknownKey(t *testing.T) {
	m := newStaticMapFeature(map[string]bool{})
	err := m.Enable("foo")
	assert.EqualError(t, err, "Unknown flag: foo")
}

func TestEnable(t *testing.T) {
	m := newStaticMapFeature(map[string]bool{"foo": false})
	err := m.Enable("foo")
	assert.NoError(t, err)
	enabled := m.IsEnabled("foo")
	assert.True(t, enabled)
}

func TestDisableUnknownKey(t *testing.T) {
	m := newStaticMapFeature(map[string]bool{})
	err := m.Disable("foo")
	assert.EqualError(t, err, "Unknown flag: foo")
}

func TestDisable(t *testing.T) {
	m := newStaticMapFeature(map[string]bool{"foo": true})
	err := m.Disable("foo")
	assert.NoError(t, err)
	enabled := m.IsEnabled("foo")
	assert.False(t, enabled)
}
