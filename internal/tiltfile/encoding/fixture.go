package encoding

import (
	"testing"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/starlarkstruct"
)

type fixture struct {
	*starkit.Fixture
}

func newFixture(t testing.TB) fixture {
	f := fixture{starkit.NewFixture(t, NewExtension(), io.NewExtension(), starlarkstruct.NewExtension())}
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
