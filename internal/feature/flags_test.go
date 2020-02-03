package feature

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestGetPanicsIfKeyDoesntExist(t *testing.T) {
	m := FeatureSet{}
	assert.Panics(t, func() {
		m.Get("foo")
	})
}

func TestIsEnabled(t *testing.T) {
	m := FeatureSet{"foo": {Enabled: true}}
	assert.True(t, m.Get("foo"))
}

func TestSetReturnsErrorOnUnknownKey(t *testing.T) {
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

func TestSetObsolete(t *testing.T) {
	m := FeatureSet{"foo": Value{Status: Obsolete, Enabled: false}}
	err := m.Set("foo", true)
	assert.EqualError(t, err, "Obsolete feature flag: foo")
}

func TestToEnabled(t *testing.T) {
	m := FeatureSet{"foo": Value{Enabled: false}}
	err := m.Set("foo", true)
	require.NoError(t, err)

	assert.Equal(t, map[string]bool{"foo": true}, m.ToEnabled())
}
