package version

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestEmpty(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.True(t, MustState(result).CheckUpdates)
}

func TestCheckUpdatesFalse(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
version_settings(check_updates=False)
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.False(t, MustState(result).CheckUpdates)
}

func TestCheckUpdatesTrue(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
version_settings(check_updates=True)
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.True(t, MustState(result).CheckUpdates)
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
