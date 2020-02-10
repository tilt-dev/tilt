package encoding

import (
	"testing"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type fixture struct {
	*starkit.Fixture
}

func newFixture(t testing.TB) fixture {
	return fixture{starkit.NewFixture(t, NewExtension(), io.NewExtension())}
}
