package dockercomposeservice

import (
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestImageIndexing(t *testing.T) {
	f := newFixture(t)
	obj := v1alpha1.DockerComposeService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.DockerComposeServiceSpec{
			ImageMaps: []string{"image-a", "image-c"},
		},
	}
	f.Create(&obj)

	// Verify we can index one image map.
	reqs := f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-a"}})
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "a"}},
	}, reqs)
}

func TestForceApply(t *testing.T) {
	f := newFixture(t)
	nn := types.NamespacedName{Name: "fe"}
	obj := v1alpha1.DockerComposeService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fe",
		},
		Spec: v1alpha1.DockerComposeServiceSpec{
			Service: "fe",
			Project: v1alpha1.DockerComposeProject{
				YAML: "fake-yaml",
			},
		},
	}
	f.Create(&obj)

	status := f.r.ForceApply(f.Context(), nn, obj.Spec, nil, false)
	assert.Equal(t, "", status.ApplyError)
	assert.Equal(t, true, status.ContainerState.Running)

	f.MustReconcile(nn)
	f.MustGet(nn, &obj)
	assert.True(t, apicmp.DeepEqual(status, obj.Status))

}

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	dcCli := dockercompose.NewFakeDockerComposeClient(t, cfb.Context())
	dCli := docker.NewFakeClient()
	clock := clockwork.NewFakeClock()
	watcher := NewDisableSubscriber(cfb.Context(), dcCli, clock)
	r := NewReconciler(cfb.Client, dcCli, dCli, cfb.Store, v1alpha1.NewScheme(), watcher)

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}
