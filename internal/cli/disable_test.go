package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisable(t *testing.T) {
	for _, tc := range []struct {
		name            string
		args            []string
		expectedEnabled []string
		expectedError   string
	}{
		{
			"normal",
			[]string{"enabled_a", "enabled_b"},
			[]string{"enabled_c", "(Tiltfile)"},
			"",
		},
		{
			"all",
			[]string{"--all"},
			[]string{"(Tiltfile)"},
			"",
		},
		{
			"all+names",
			[]string{"--all", "enabled_b"},
			nil,
			"cannot use --all with resource names",
		},
		{
			"no names",
			nil,
			nil,
			"must specify at least one resource",
		},
		{
			"nonexistent resource",
			[]string{"foo"},
			nil,
			"no such resource \"foo\"",
		},
		{
			"Tiltfile",
			[]string{"(Tiltfile)"},
			nil,
			"(Tiltfile) cannot be enabled or disabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEnableFixture(t)
			defer f.TearDown()

			f.createResources()

			cmd := disableCmd{}
			c := cmd.register()
			err := c.Flags().Parse(tc.args)
			require.NoError(t, err)
			err = cmd.run(f.ctx, c.Flags().Args())
			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				// if there's an error, expect enabled states to remain the same
				tc.expectedEnabled = []string{"enabled_a", "enabled_b", "enabled_c", "(Tiltfile)"}
			} else {
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedEnabled, f.enabledResources())
		})
	}
}
