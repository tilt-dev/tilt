package cluster

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tilt-dev/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/timecmp"
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
	f.r.connManager.store(nn, connection{
		spec:  cluster.Spec,
		error: "connection error",
	})
	f.Create(cluster)

	assert.Equal(t, "", cluster.Status.Error)
	f.MustGet(nn, cluster)
	assert.Equal(t, "connection error", cluster.Status.Error)
	assert.Nil(t, cluster.Status.ConnectedAt, "ConnectedAt should be empty")

	f.assertSteadyState(cluster)
}

func TestKubernetesArch(t *testing.T) {
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
	client := k8s.NewFakeK8sClient(t)
	client.Inject(k8s.K8sEntity{
		Obj: &v1.Node{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				UID:  "a",
				Labels: map[string]string{
					"kubernetes.io/arch": "amd64",
				},
			},
		},
	})
	f.r.SetFakeClientsForTesting(client, nil)

	f.Create(cluster)
	f.MustGet(nn, cluster)
	assert.Equal(t, "amd64", cluster.Status.Arch)

	f.assertSteadyState(cluster)

	connectEvt := analytics.CountEvent{
		Name: "api.cluster.connect",
		Tags: map[string]string{
			"type":   "kubernetes",
			"arch":   "amd64",
			"status": "connected",
		},
		N: 1,
	}
	assert.ElementsMatch(t, []analytics.CountEvent{connectEvt}, f.ma.Counts)
}

func TestDockerArch(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Docker: &v1alpha1.DockerClusterConnection{},
			},
		},
	}

	// Create a fake client.
	nn := types.NamespacedName{Name: "default"}
	client := docker.NewFakeClient()
	createdAt := time.Now()
	f.r.connManager.store(nn, connection{
		spec:         cluster.Spec,
		dockerClient: client,
		createdAt:    createdAt,
	})
	f.Create(cluster)
	f.MustGet(nn, cluster)
	assert.Equal(t, "amd64", cluster.Status.Arch)
	timecmp.AssertTimeEqual(t, createdAt, cluster.Status.ConnectedAt)
}

type fixture struct {
	*fake.ControllerFixture
	r  *Reconciler
	ma *analytics.MemoryAnalytics
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	st := store.NewTestingStore()

	r := NewReconciler(cfb.Context(), cfb.Client, st, docker.LocalEnv{}, NewConnectionManager())
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		ma:                cfb.Analytics(),
	}
}

func (f *fixture) assertSteadyState(o *v1alpha1.Cluster) {
	f.T().Helper()
	f.MustReconcile(types.NamespacedName{Name: o.Name})
	var o2 v1alpha1.Cluster
	f.MustGet(types.NamespacedName{Name: o.Name}, &o2)
	assert.True(f.T(), apicmp.DeepEqual(o, &o2), cmp.Diff(o, &o2),
		"Cluster object should have been in steady state but changed")
}
