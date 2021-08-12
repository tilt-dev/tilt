package telemetry

import (
	"testing"
	"time"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestTelemetryCmdString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_telemetry_cmd('foo.sh')")
	result, err := f.ExecFile("Tiltfile")

	assert.NoError(t, err)
	assert.Equal(t, model.ToHostCmdInDir("foo.sh", f.Path()), MustState(result).Cmd)
	assert.Equal(t, model.DefaultTelemetryPeriod, MustState(result).Period)
}

func TestTelemetryPeriod(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_telemetry_cmd('foo.sh', period='5s')")

	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Second, MustState(result).Period)
}

func TestTelemetryCmdArray(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_telemetry_cmd(['foo.sh'])")
	result, err := f.ExecFile("Tiltfile")

	assert.NoError(t, err)
	assert.Equal(t, model.Cmd{Argv: []string{"foo.sh"}, Dir: f.Path()}, MustState(result).Cmd)
}

func TestTelemetryCmdEmpty(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_telemetry_cmd('')")
	_, err := f.ExecFile("Tiltfile")

	assert.EqualError(t, err, "cmd cannot be empty")
}

func TestTelemetryCmdMultiple(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", `
experimental_telemetry_cmd('foo.sh')
experimental_telemetry_cmd('bar.sh')
`)
	_, err := f.ExecFile("Tiltfile")
	assert.EqualError(t, err, "experimental_telemetry_cmd called multiple times; already set to foo.sh")
}

func newFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewPlugin())
}
