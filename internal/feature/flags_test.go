package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReturnsFalseIfKeyDoesntExist(t *testing.T) {
	m := FeatureSet{}
	assert.False(t, m.Get("foo"))
}

func TestIsEnabled(t *testing.T) {
	m := FeatureSet{"foo": {Enabled: true}}
	assert.True(t, m.Get("foo"))
}

func TestSetReturnsErrorForUnknownKey(t *testing.T) {
	m := FeatureSet{}
	err := m.Set("foo", false)
	assert.EqualError(t, err, "Unknown feature flag: foo")
}

func TestSetSimple(t *testing.T) {
	m := FeatureSet{"foo": Value{Enabled: false}}
	err := m.Set("foo", true)
	assert.NoError(t, err)
	assert.True(t, m.Get("foo"))
}

func TestSetNoop(t *testing.T) {
	m := FeatureSet{"foo": Value{Status: Noop, Enabled: false}}
	err := m.Set("foo", true)
	assert.NoError(t, err)
	assert.False(t, m.Get("foo"))
}

func TestSetWarn(t *testing.T) {
	m := FeatureSet{"foo": Value{Status: Warn, Enabled: false}}
	err := m.Set("foo", true)
	assert.EqualError(t, err, "Obsolete feature flag: foo")
}

func TestToEnabled(t *testing.T) {
	m := FeatureSet{"foo": Value{Enabled: false}}
	m.Set("foo", true)

	assert.Equal(t, map[string]bool{"foo": true}, m.ToEnabled())
}
