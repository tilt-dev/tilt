package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestMetricsEnabled(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_metrics_settings(enabled=True)")
	result, err := f.ExecFile("Tiltfile")

	assert.NoError(t, err)
	assert.True(t, MustState(result).Enabled)
	assert.Equal(t, "opentelemetry.tilt.dev:443", MustState(result).Address)
	assert.False(t, MustState(result).Insecure)
	assert.Equal(t, model.DefaultReportingPeriod, MustState(result).ReportingPeriod)
}

func TestMetricsAddress(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
experimental_metrics_settings(enabled=True)
experimental_metrics_settings(address='localhost:5678')
experimental_metrics_settings(insecure=True)
experimental_metrics_settings(reporting_period='1s')
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.True(t, MustState(result).Enabled)
	assert.Equal(t, "localhost:5678", MustState(result).Address)
	assert.True(t, MustState(result).Insecure)
	assert.Equal(t, time.Second, MustState(result).ReportingPeriod)
}

func newFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewPlugin())
}
