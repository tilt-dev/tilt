package dockerimage

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestIndexCluster(t *testing.T) {
	f := newFixture(t)

	f.Create(&v1alpha1.DockerImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-image",
		},
		Spec: v1alpha1.DockerImageSpec{
			Cluster: "cluster",
		},
	})

	reqs := f.r.indexer.Enqueue(&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}})
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "my-image"}},
	}, reqs, "Index result for known cluster")

	reqs = f.r.indexer.Enqueue(&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "other"}})
	require.Empty(t, reqs, "Index result for unknown cluster")
}

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t testing.TB) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)

	r := NewReconciler(cfb.Client, cfb.Scheme())
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}
