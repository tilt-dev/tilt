package encoding

import (
	"testing"

	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/starlarkstruct"
)

type fixture struct {
	*starkit.Fixture
}

func newFixture(t testing.TB) fixture {
	f := fixture{starkit.NewFixture(t, NewPlugin(), io.NewPlugin(), starlarkstruct.NewPlugin())}
	f.UseRealFS()
	f.File("assert.tilt", `
def equals(expected, observed):
	if expected != observed:
		print("expected: '%s'" % (expected))
		print("observed: '%s'" % (observed))
		fail()

assert = struct(equals=equals)
`)
	return f
}
