package telemetry

import (
	"testing"

	"github.com/windmilleng/tilt/pkg/model"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestTelemetryCmdString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_telemetry_cmd('foo.sh')")
	result, err := f.ExecFile("Tiltfile")

	assert.NoError(t, err)
	assert.Equal(t, model.ToShellCmd("foo.sh"), MustState(result).Cmd)
}

func TestTelemetryCmdArray(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "experimental_telemetry_cmd(['foo.sh'])")
	result, err := f.ExecFile("Tiltfile")

	assert.NoError(t, err)
	assert.Equal(t, model.Cmd{Argv: []string{"foo.sh"}}, MustState(result).Cmd)
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
	return starkit.NewFixture(tb, NewExtension())
}
