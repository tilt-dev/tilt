package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHudEnabled(t *testing.T) {
	for _, test := range []struct {
		name             string
		args             string
		expectHUDEnabled bool
	}{
		{"old behavior: no --hud", "--default-hud", true},
		{"old behavior: --hud", "--default-hud --hud", true},
		{"old behavior: --hud=false", "--default-hud --hud=false", false},
		{"new behavior: no --hud", "--default-hud=false", false},
		{"new behavior: --hud", "--default-hud=false --hud", true},
		{"new behavior: --hud=false", "--default-hud=false --hud=false", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := upCmd{}

			args := strings.Split(test.args, " ")

			c := cmd.register()
			err := c.Flags().Parse(args)
			require.NoError(t, err)

			c.PreRun(c, args)

			require.Equal(t, test.expectHUDEnabled, cmd.isHudEnabledByConfig(), test.args)
		})
	}
}
