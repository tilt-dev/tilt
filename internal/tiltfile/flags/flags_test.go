package flags

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestEmpty(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Equal(t, 0, len(MustState(result).Resources))
}

func TestNonEmpty(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
flags.set_resources(['foo', 'bar', 'baz'])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar", "baz"}, MustState(result).Resources)
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
