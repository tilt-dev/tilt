package cluster

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestKubernetesError(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
	}

	// Create a fake client.
	nn := types.NamespacedName{Name: "default"}
	f.r.connections[nn] = &connection{
		spec:  cluster.Spec,
		error: "connection error",
	}
	f.Create(cluster)

	assert.Equal(t, "", cluster.Status.Error)
	f.MustGet(nn, cluster)
	assert.Equal(t, "connection error", cluster.Status.Error)

	f.assertSteadyState(cluster)
}

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	st := store.NewTestingStore()

	r := NewReconciler(cfb.Client, st, docker.LocalEnv{})
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}

func (f *fixture) assertSteadyState(o *v1alpha1.Cluster) {
	f.T().Helper()
	f.MustReconcile(types.NamespacedName{Name: o.Name})
	var o2 v1alpha1.Cluster
	f.MustGet(types.NamespacedName{Name: o.Name}, &o2)
	assert.Equal(f.T(), o.ResourceVersion, o2.ResourceVersion)
}
