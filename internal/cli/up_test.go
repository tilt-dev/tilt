package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/testutils"
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
