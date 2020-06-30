package tilt

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestSubCommand(t *testing.T) {
	f := NewFixture(t, "foo")
	defer f.TearDown()

	f.File("Tiltfile", `
print(tilt.sub_command)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Equal(t, "foo\n", f.PrintOutput())
}

func NewFixture(tb testing.TB, subcommand TiltSubcommand) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension(subcommand))
}
