package cli

import (
	"testing"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"

	"github.com/stretchr/testify/require"
)

func TestSubCommandSet(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", "print('hi')")
	cmd := Cmd()
	cmd.SetArgs([]string{"alpha", "tiltfile-result", "--file", f.JoinPath("Tiltfile")})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "alpha tiltfile-result", subcommand)
}
