package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/engine/analytics"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestHudWiring(t *testing.T) {
	ctx, _, ta := testutils.CtxAndAnalyticsForTest()

	tests := []struct {
		name       string
		hudEnabled bool
	}{
		{name: "hud enabled", hudEnabled: true},
		{name: "hud disabled", hudEnabled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			threads, err := wireCmdUp(ctx, hud.HudEnabled(test.hudEnabled), ta, analytics.CmdTags{})
			if err != nil {
				t.Fatal(err)
			}

			var expectedType hud.HeadsUpDisplay
			if test.hudEnabled {
				expectedType = &hud.Hud{}
			} else {
				expectedType = &hud.DisabledHud{}
			}

			assert.IsType(t, expectedType, threads.Hud)
		})
	}
}

func TestHudEnabled(t *testing.T) {
	for _, test := range []struct {
		name             string
		args             string
		expectHUDEnabled bool
	}{
		{"old behavior: no --hud", "--default-tui", true},
		{"old behavior: --hud", "--default-tui --hud", true},
		{"old behavior: --hud=false", "--default-tui --hud=false", false},
		{"new behavior: no --hud", "--default-tui=false", false},
		{"new behavior: --hud", "--default-tui=false --hud", true},
		{"new behavior: --hud=false", "--default-tui=false --hud=false", false},
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
