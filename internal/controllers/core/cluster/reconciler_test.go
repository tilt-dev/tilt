package cluster

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/xdg"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/timecmp"
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
	origClientFactory := f.r.k8sClientFactory
	f.r.k8sClientFactory = FakeKubernetesClientOrError(nil, errors.New("fake error"))
	f.Create(cluster)

	assert.Equal(t, "", cluster.Status.Error)
	f.MustGet(nn, cluster)
	assert.Equal(t,
		"Tilt encountered an error connecting to your Kubernetes cluster:\n\tfake error\nYou will need to restart Tilt after resolving the issue.",
		cluster.Status.Error)
	assert.Nil(t, cluster.Status.ConnectedAt, "ConnectedAt should be empty")

	// replace the working client factory but ensure that it's not invoked
	// we should be in a steady state until the retry/backoff window elapses
	f.r.k8sClientFactory = origClientFactory
	f.assertSteadyState(cluster)

	// advance the clock such that we should retry, but ensure that no retry
	// is attempted because the cluster refresh feature flag annotation is
	// not set
	f.clock.Advance(time.Minute)
	f.assertSteadyState(cluster)

	// add the cluster refresh feature flag and verify that it gets refreshed
	// and creates a new client without errors
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations["features.tilt.dev/cluster-refresh"] = "true"
	f.Update(cluster)

	f.MustGet(nn, cluster)
	require.Empty(t, cluster.Status.Error, "No error should be present on cluster")
	if assert.NotNil(t, cluster.Status.ConnectedAt, "ConnectedAt should be populated") {
		assert.NotZero(t, cluster.Status.ConnectedAt.Time, "ConnectedAt should not be zero time")
	}
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

func TestKubernetesConnStatus(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
	}

	nn := types.NamespacedName{Name: "default"}
	f.Create(cluster)
	f.MustGet(nn, cluster)

	configPath := cluster.Status.Connection.Kubernetes.ConfigPath
	require.NotEqual(t, configPath, "")

	expected := &v1alpha1.ClusterConnectionStatus{
		Kubernetes: &v1alpha1.KubernetesClusterConnectionStatus{
			Context:    "default",
			Namespace:  "default",
			Cluster:    "default",
			Product:    "unknown",
			ConfigPath: configPath,
		},
	}
	assert.Equal(t, expected, cluster.Status.Connection)

	contents, err := afero.ReadFile(f.fs, configPath)
	require.NoError(t, err)
	assert.Equal(t, `apiVersion: v1
clusters:
- cluster:
    server: ""
  name: default
contexts:
- context:
    cluster: default
    namespace: default
    user: ""
  name: default
current-context: default
kind: Config
preferences: {}
users: null
`, string(contents))
}

func TestKubernetesMonitor(t *testing.T) {
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
	f.MustGet(nn, cluster)
	connectedAt := *cluster.Status.ConnectedAt
	f.assertSteadyState(cluster)

	f.k8sClient.ClusterHealthError = errors.New("fake cluster health error")
	f.clock.Advance(time.Minute)
	<-f.requeues

	f.MustGet(nn, cluster)
	assert.Equal(t, "fake cluster health error", cluster.Status.Error)
	timecmp.RequireTimeEqual(t, connectedAt, cluster.Status.ConnectedAt)
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

func TestKubeconfig_RuntimeDirImmutable(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
	}

	p, err := f.base.RuntimeFile(filepath.Join("tilt-default", "cluster", "default.yml"))
	require.NoError(t, err)
	runtimeFile, _ := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0400)
	_ = runtimeFile.Close()

	nn := types.NamespacedName{Name: "default"}
	f.Create(cluster)
	f.MustGet(nn, cluster)

	configPath := cluster.Status.Connection.Kubernetes.ConfigPath
	require.NotEqual(t, configPath, "")
}

func TestKubeconfig_RuntimeAndStateDirImmutable(t *testing.T) {
	f := newFixture(t)
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
	}

	p, err := f.base.RuntimeFile(filepath.Join("tilt-default", "cluster", "default.yml"))
	require.NoError(t, err)
	runtimeFile, _ := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0400)
	_ = runtimeFile.Close()

	p, err = f.base.StateFile(filepath.Join("tilt-default", "cluster", "default.yml"))
	require.NoError(t, err)
	stateFile, _ := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0400)
	_ = stateFile.Close()

	nn := types.NamespacedName{Name: "default"}
	f.Create(cluster)
	f.MustGet(nn, cluster)

	require.Equal(t, cluster.Status.Connection.Kubernetes.ConfigPath, "")
	require.Contains(t, cluster.Status.Error, "storing temp kubeconfigs")
}

type fixture struct {
	*fake.ControllerFixture
	r            *Reconciler
	ma           *analytics.MemoryAnalytics
	clock        clockwork.FakeClock
	k8sClient    *k8s.FakeK8sClient
	dockerClient *docker.FakeClient
	base         *xdg.FakeBase
	requeues     <-chan indexer.RequeueForTestResult
	fs           afero.Fs
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	clock := clockwork.NewFakeClock()
	tmpf := tempdir.NewTempDirFixture(t)

	k8sClient := k8s.NewFakeK8sClient(t)
	dockerClient := docker.NewFakeClient()
	fs := afero.NewOsFs()
	base := xdg.NewFakeBase(tmpf.Path(), fs)
	r := NewReconciler(cfb.Context(),
		cfb.Client,
		cfb.Store,
		clock,
		NewConnectionManager(),
		docker.LocalEnv{},
		FakeDockerClientOrError(dockerClient, nil),
		FakeKubernetesClientOrError(k8sClient, nil),
		server.NewWebsocketList(),
		base,
		"tilt-default",
		fs)
	requeueChan := make(chan indexer.RequeueForTestResult, 1)
	return &fixture{
		ControllerFixture: cfb.WithRequeuer(r.requeuer).WithRequeuerResultChan(requeueChan).Build(r),
		r:                 r,
		ma:                cfb.Analytics(),
		clock:             clock,
		k8sClient:         k8sClient,
		dockerClient:      dockerClient,
		requeues:          requeueChan,
		base:              base,
		fs:                fs,
	}
}

func (f *fixture) assertSteadyState(o *v1alpha1.Cluster) {
	f.T().Helper()
	f.MustReconcile(types.NamespacedName{Name: o.Name})
	var o2 v1alpha1.Cluster
	f.MustGet(types.NamespacedName{Name: o.Name}, &o2)
	assert.True(f.T(), apicmp.DeepEqual(o, &o2),
		"Cluster object should have been in steady state but changed: %s",
		cmp.Diff(o, &o2))
}
