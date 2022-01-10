package cluster

import (
	"errors"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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
	mu      sync.Mutex
	clients map[types.NamespacedName]clientOrErr
}

var _ ClientProvider = &FakeClientProvider{}

// NewFakeClientProvider creates a client provider suitable for tests.
//
// If defaultClient is not nil, it will be immediately available for the "default" Cluster connection.
// It's possible to store additional clients for other Cluster connections as well.
func NewFakeClientProvider(defaultClient k8s.Client) *FakeClientProvider {
	fcc := &FakeClientProvider{
		clients: make(map[types.NamespacedName]clientOrErr),
	}

	if defaultClient != nil {
		defaultNN := types.NamespacedName{Name: v1alpha1.ClusterNameDefault}
		fcc.AddK8sClient(defaultNN, defaultClient)
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
