package cluster

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type clientOrErr struct {
	clientRevision
	err error
}

type FakeClientProvider struct {
	t          testing.TB
	mu         sync.Mutex
	clients    map[types.NamespacedName]clientOrErr
	ctrlClient ctrlclient.Client
}

var _ ClientProvider = &FakeClientProvider{}

// NewFakeClientProvider creates a client provider suitable for tests.
func NewFakeClientProvider(t testing.TB, ctrlClient ctrlclient.Client) *FakeClientProvider {
	fcc := &FakeClientProvider{
		t:          t,
		ctrlClient: ctrlClient,
		clients:    make(map[types.NamespacedName]clientOrErr),
	}

	return fcc
}

func (f *FakeClientProvider) GetK8sClient(clusterKey types.NamespacedName) (k8s.Client, metav1.MicroTime, error) {
	if clusterKey.Name == "" {
		return nil, metav1.MicroTime{}, errors.New("cluster key cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.clients[clusterKey]
	if !ok {
		return nil, metav1.MicroTime{}, NotFoundError
	}

	if c.err != nil {
		// intentionally erase the error type
		return nil, metav1.MicroTime{}, errors.New(c.err.Error())
	}

	return c.client, c.connectedAt, nil
}

// AddK8sClient adds the client if there is currently no client/error for the cluster key.
func (f *FakeClientProvider) AddK8sClient(key types.NamespacedName, client k8s.Client) (bool, metav1.MicroTime) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.clients[key]; !ok {
		now := apis.NowMicro()
		f.clients[key] = clientOrErr{clientRevision: clientRevision{client: client, connectedAt: now}}
		return true, now
	}
	return false, metav1.MicroTime{}
}

// SetK8sClient sets a client for the cluster key, overwriting any that exists.
func (f *FakeClientProvider) SetK8sClient(key types.NamespacedName, client k8s.Client) metav1.MicroTime {
	f.mu.Lock()
	defer f.mu.Unlock()

	// in apiserver, it's not feasible for a client to get updated repeatedly
	// at sub-microsecond level speed, but this ensures things play nicely in
	// tests by making the timestamp always move forward
	now := metav1.NowMicro()
	if existing, ok := f.clients[key]; ok {
		if timecmp.BeforeOrEqual(now, existing.connectedAt) {
			now = apis.NewMicroTime(existing.connectedAt.Add(time.Microsecond))
		}
	}

	f.clients[key] = clientOrErr{clientRevision: clientRevision{client: client, connectedAt: now}}
	return now
}

// SetClusterError sets an error for the cluster key.
func (f *FakeClientProvider) SetClusterError(key types.NamespacedName, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.clients[key] = clientOrErr{err: err}
}

func (f *FakeClientProvider) MustK8sClient(clusterNN types.NamespacedName) *k8s.FakeK8sClient {
	f.t.Helper()
	kCli, _, err := f.GetK8sClient(clusterNN)
	require.NoError(f.t, err,
		"Maybe you forgot to call FakeClientProvider::EnsureK8sCluster?")
	require.IsType(f.t, &k8s.FakeK8sClient{}, kCli,
		"Only *k8s.FakeK8sClient should exist in the provider")
	return kCli.(*k8s.FakeK8sClient)
}

func (f *FakeClientProvider) EnsureK8sClusterError(ctx context.Context, clusterNN types.NamespacedName,
	clusterErr error) {
	f.t.Helper()

	f.SetClusterError(clusterNN, clusterErr)

	f.upsertClusterStatus(ctx, clusterNN,
		v1alpha1.ClusterStatus{
			ConnectedAt: nil,
			Error:       clusterErr.Error(),
		})
}

func (f *FakeClientProvider) EnsureDefaultK8sCluster(ctx context.Context) *k8s.FakeK8sClient {
	kCli, _ := f.EnsureK8sCluster(ctx, types.NamespacedName{Name: "default"})
	return kCli
}

func (f *FakeClientProvider) EnsureK8sCluster(
	ctx context.Context,
	clusterNN types.NamespacedName,
) (*k8s.FakeK8sClient, metav1.MicroTime) {
	f.t.Helper()

	kCli, rev, err := f.GetK8sClient(clusterNN)
	if err != nil {
		fakeCli := k8s.NewFakeK8sClient(f.t)
		rev = f.SetK8sClient(clusterNN, fakeCli)
		kCli = fakeCli
	}

	f.upsertClusterStatus(ctx, clusterNN,
		v1alpha1.ClusterStatus{
			Arch:        "amd64",
			Version:     "1.23.5",
			ConnectedAt: &rev,
			Connection: &v1alpha1.ClusterConnectionStatus{
				Kubernetes: &v1alpha1.KubernetesClusterConnectionStatus{
					Product: "kind",
				},
			},
		})
	return kCli.(*k8s.FakeK8sClient), rev
}

func (f *FakeClientProvider) upsertClusterStatus(ctx context.Context, clusterNN types.NamespacedName,
	status v1alpha1.ClusterStatus) {
	f.t.Helper()
	var cluster v1alpha1.Cluster
	err := f.ctrlClient.Get(ctx, clusterNN, &cluster)
	if apierrors.IsNotFound(err) {
		cluster.ObjectMeta = metav1.ObjectMeta{
			Namespace: clusterNN.Namespace,
			Name:      clusterNN.Name,
		}
		require.NoError(f.t, f.ctrlClient.Create(ctx, &cluster))
	}
	if !apicmp.DeepEqual(cluster.Status, status) {
		cluster.Status = status
		require.NoError(f.t, f.ctrlClient.Status().Update(ctx, &cluster))
	}
}
