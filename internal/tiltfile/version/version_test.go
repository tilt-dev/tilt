package version

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestEmpty(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.True(t, MustState(result).CheckUpdates)
}

func TestCheckUpdatesFalse(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
version_settings(check_updates=False)
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.False(t, MustState(result).CheckUpdates)
}

func TestCheckUpdatesTrue(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
version_settings(check_updates=True)
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.True(t, MustState(result).CheckUpdates)
}

func TestVersionConstraints(t *testing.T) {
	for _, tc := range []struct {
		constraint string
		meets      bool
	}{
		{">=0.5.4", false},
		{"<=0.5.2", false},
		{"<=0.3.0", false},
		{">=0.5.0 <0.5.3", false},
		{"=0.5.3", true},
		{">0.4.1", true},
		{"0.5.x", true},
	} {
		var verb string
		if tc.meets {
			verb = "meets"
		} else {
			verb = "doesn't meet"
		}
		t.Run(fmt.Sprintf("version %s constraint %s", verb, tc.constraint), func(t *testing.T) {
			f := NewFixture(t)
			f.File("Tiltfile", fmt.Sprintf(`
version_settings(constraint='%s')
`, tc.constraint))
			_, err := f.ExecFile("Tiltfile")
			if tc.meets {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), TestingVersion)
				require.Contains(t, err.Error(), fmt.Sprintf("'%s'", tc.constraint))
			}
		})
	}
}

func TestVersionConstraintsDontClearCheckUpdates(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
version_settings(check_updates=False)
version_settings(constraint='0.x')
`)

	m, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	var settings model.VersionSettings
	err = m.Load(&settings)
	require.NoError(t, err)
	require.False(t, settings.CheckUpdates)
}

const TestingVersion = "0.5.3"

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension(model.TiltBuild{Version: TestingVersion}))
}
