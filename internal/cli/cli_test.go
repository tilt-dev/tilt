package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubCommandSet(t *testing.T) {
	cmd := Cmd()
	cmd.SetArgs([]string{"alpha", "tiltfile-result"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "alpha tiltfile-result", subcommand)
}
