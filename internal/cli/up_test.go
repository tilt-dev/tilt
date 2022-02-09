package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/store"
)

func TestHudEnabled(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     string
		expected store.TerminalMode
	}{
		{"old behavior: no --hud", "", store.TerminalModePrompt},
		{"old behavior: --legacy", "--legacy", store.TerminalModeHUD},
		{"old behavior: --stream=true", "--stream=true", store.TerminalModeStream},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := upCmd{}

			args := strings.Split(test.args, " ")

			c := cmd.register()
			err := c.Flags().Parse(args)
			require.NoError(t, err)

			c.PreRun(c, args)

			require.Equal(t, test.expected, cmd.initialTermMode(true), test.args)
		})
	}
}
