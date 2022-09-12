package k8scontext

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestK8sNamespaceDefaultNamespace(t *testing.T) {
	f := NewFixture(t, "gke-blorg", "default", clusterid.ProductGKE)
	f.File("Tiltfile", `
print(k8s_namespace())
`)
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)

	assert.Equal(t, "default\n", f.PrintOutput())
}

func TestK8sNamespaceNonStandardNamespace(t *testing.T) {
	f := NewFixture(t, "gke-blorg", "im-a-teapot", clusterid.ProductGKE)
	f.File("Tiltfile", `
print(k8s_namespace())
`)
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)

	assert.Equal(t, "im-a-teapot\n", f.PrintOutput())
}

func TestAllowK8sContext(t *testing.T) {
	f := NewFixture(t, "gke-blorg", "default", clusterid.ProductGKE)
	f.File("Tiltfile", `
allow_k8s_contexts('gke-blorg')
`)
	model, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, []k8s.KubeContext{"gke-blorg"}, MustState(model).allowed)
	assert.True(t, MustState(model).IsAllowed(f.Tiltfile()))

	model, err = f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, []k8s.KubeContext{"gke-blorg"}, MustState(model).allowed)
}

func TestForbidK8sContext(t *testing.T) {
	f := NewFixture(t, "gke-blorg", "default", clusterid.ProductGKE)
	f.File("Tiltfile", `
`)
	model, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.False(t, MustState(model).IsAllowed(f.Tiltfile()))

	// All k8s contexts are allowed in extensions.
	f.Tiltfile().ObjectMeta.Name = "my-ext"
	assert.True(t, MustState(model).IsAllowed(f.Tiltfile()))
}

func NewFixture(tb testing.TB, ctx k8s.KubeContext, ns k8s.Namespace, env clusterid.Product) *starkit.Fixture {
	return starkit.NewFixture(tb, NewPlugin(ctx, ns, env))
}
