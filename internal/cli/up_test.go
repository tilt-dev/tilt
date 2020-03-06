package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/engine/analytics"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestHudEnabled(t *testing.T) {
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
			threads, err := wireCmdUp(ctx, hud.HudEnabled(test.hudEnabled), ta, analytics.CmdUpTags{})
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
