package cisettings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestK8sGracePeriod(t *testing.T) {
	f := newFixture(t)
	f.File("Tiltfile", `
ci_settings(k8s_grace_period='3m')
`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	ci, err := GetState(result)
	require.NoError(t, err)
	require.Equal(t, 3*time.Minute, ci.K8sGracePeriod.Duration)
	require.Equal(t, model.CITimeoutDefault, ci.Timeout.Duration)
}

func TestK8sGracePeriodOverride(t *testing.T) {
	f := newFixture(t)
	f.File("Tiltfile", `
ci_settings(k8s_grace_period='3m')
ci_settings(k8s_grace_period='5s')
`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	ci, err := GetState(result)
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, ci.K8sGracePeriod.Duration)
}

func TestK8sGracePeriodOverrideEmpty(t *testing.T) {
	f := newFixture(t)
	f.File("Tiltfile", `
ci_settings(k8s_grace_period='3m')
ci_settings()
`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	ci, err := GetState(result)
	require.NoError(t, err)
	require.Equal(t, 3*time.Minute, ci.K8sGracePeriod.Duration)
}

func TestTimeout(t *testing.T) {
	f := newFixture(t)
	f.File("Tiltfile", `
ci_settings(timeout='3m')
ci_settings()
`)

	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	ci, err := GetState(result)
	require.NoError(t, err)
	require.Equal(t, 3*time.Minute, ci.Timeout.Duration)
}

func newFixture(t testing.TB) *starkit.Fixture {
	return starkit.NewFixture(t, NewPlugin(model.CITimeoutFlag(model.CITimeoutDefault)))
}
