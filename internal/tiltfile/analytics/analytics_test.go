package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
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

func TestReportToAnalytics(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
experimental_analytics_report({'1': '2'})
# the second call's "1" value replaces the first
experimental_analytics_report({'1': '2a', '3': '4'})
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"1": "2a", "3": "4"}, MustState(result).CustomTagsToReport)
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewPlugin())
}
