package k8scontext

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestAllowK8sContext(t *testing.T) {
	f, ext := NewFixture(t, "gke-blorg", k8s.EnvGKE)

	f.File("Tiltfile", `
allow_k8s_contexts('gke-blorg')
`)
	err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, []k8s.KubeContext{"gke-blorg"}, ext.allowedK8sContexts)
	assert.True(t, ext.IsAllowed())

	err = f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, []k8s.KubeContext{"gke-blorg"}, ext.allowedK8sContexts)
}

func NewFixture(tb testing.TB, ctx k8s.KubeContext, env k8s.Env) (*starkit.Fixture, *Extension) {
	ext := NewExtension(ctx, env)
	return starkit.NewFixture(tb, ext), ext
}
