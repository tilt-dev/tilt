package k8scontext

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestAllowK8sContext(t *testing.T) {
	f := NewFixture(t, "gke-blorg", k8s.EnvGKE)
	f.File("Tiltfile", `
allow_k8s_contexts('gke-blorg')
`)
	model, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, []k8s.KubeContext{"gke-blorg"}, MustState(model).allowed)
	assert.True(t, MustState(model).IsAllowed())

	model, err = f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, []k8s.KubeContext{"gke-blorg"}, MustState(model).allowed)
}

func NewFixture(tb testing.TB, ctx k8s.KubeContext, env k8s.Env) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension(ctx, env))
}
