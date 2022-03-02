package dockercomposeservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
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

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	r := NewReconciler(cfb.Client, cfb.Store, v1alpha1.NewScheme())

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}
