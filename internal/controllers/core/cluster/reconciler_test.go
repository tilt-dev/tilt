package cluster

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
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
	nn := apis.Key(cluster)

	// Create a fake client factory that always returns an error
	f.r.k8sClientFactory = FakeKubernetesClientOrError(nil, errors.New("connection error"))
	f.Create(cluster)

	assert.Equal(t, "", cluster.Status.Error)
	f.MustGet(nn, cluster)
	assert.Equal(t, "connection error", cluster.Status.Error)
	assert.Nil(t, cluster.Status.ConnectedAt, "ConnectedAt should be empty")

	f.assertSteadyState(cluster)
}

func TestKubernetesDelete(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
	}
	nn := apis.Key(cluster)

	f.Create(cluster)
	_, ok := f.r.connManager.load(nn)
	require.True(t, ok, "Connection was not present in connection manager")

	f.Delete(cluster)
	_, ok = f.r.connManager.load(nn)
	require.False(t, ok, "Connection was not removed from connection manager")
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

	// Inject a Node into the fake client so that the arch can be determined.
	nn := types.NamespacedName{Name: "default"}
	f.k8sClient.Inject(k8s.K8sEntity{
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

func TestDockerError(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Docker: &v1alpha1.DockerClusterConnection{},
			},
		},
	}
	nn := apis.Key(cluster)

	f.r.dockerClientFactory = FakeDockerClientOrError(nil, errors.New("fake docker error"))

	f.Create(cluster)
	f.MustGet(nn, cluster)
	assert.Equal(t, "fake docker error", cluster.Status.Error)
	assert.Nil(t, cluster.Status.ConnectedAt, "ConnectedAt should not be populated")
	assert.Empty(t, cluster.Status.Arch, "no arch should be present")
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

	nn := types.NamespacedName{Name: "default"}
	f.Create(cluster)
	f.MustGet(nn, cluster)
	assert.Equal(t, "amd64", cluster.Status.Arch)
	if assert.NotNil(t, cluster.Status.ConnectedAt, "ConnectedAt should be populated") {
		assert.NotZero(t, cluster.Status.ConnectedAt.Time, "ConnectedAt should not be zero")
	}
}

type fixture struct {
	*fake.ControllerFixture
	r            *Reconciler
	ma           *analytics.MemoryAnalytics
	k8sClient    *k8s.FakeK8sClient
	dockerClient *docker.FakeClient
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	st := store.NewTestingStore()

	k8sClient := k8s.NewFakeK8sClient(t)
	dockerClient := docker.NewFakeClient()

	r := NewReconciler(cfb.Context(), cfb.Client, st, NewConnectionManager(), docker.LocalEnv{},
		FakeDockerClientOrError(dockerClient, nil),
		FakeKubernetesClientOrError(k8sClient, nil))
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		ma:                cfb.Analytics(),
		k8sClient:         k8sClient,
		dockerClient:      dockerClient,
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
