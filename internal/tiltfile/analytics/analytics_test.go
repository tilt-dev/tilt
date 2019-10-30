package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestEmpty(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, analytics.OptDefault, MustState(result).Opt)
}

func TestEnableTrue(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
analytics_settings(enable=True)
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, analytics.OptIn, MustState(result).Opt)
}

func TestEnableFalse(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
analytics_settings(enable=False)
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, analytics.OptOut, MustState(result).Opt)
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
