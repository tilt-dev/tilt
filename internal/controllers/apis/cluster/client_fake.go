package cluster

import (
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type clientOrErr struct {
	watermarkedClient
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

func (f *FakeClientProvider) GetK8sClient(clusterKey types.NamespacedName) (k8s.Client, time.Time, error) {
	if clusterKey.Name == "" {
		return nil, time.Time{}, errors.New("cluster key cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.clients[clusterKey]
	if !ok {
		return nil, time.Time{}, NotFoundError
	}

	if c.err != nil {
		// intentionally erase the error type
		return nil, time.Time{}, errors.New(c.err.Error())
	}

	return c.client, c.connectedAt, nil
}

// AddK8sClient adds the client if there is currently no client/error for the cluster key.
func (f *FakeClientProvider) AddK8sClient(key types.NamespacedName, client k8s.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.clients[key]; !ok {
		f.clients[key] = clientOrErr{watermarkedClient: watermarkedClient{client: client, connectedAt: time.Now()}}
	}
}

// SetK8sClient sets a client for the cluster key, overwriting any that exists.
func (f *FakeClientProvider) SetK8sClient(key types.NamespacedName, client k8s.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.clients[key] = clientOrErr{watermarkedClient: watermarkedClient{client: client, connectedAt: time.Now()}}
}

// SetClusterError sets an error for the cluster key.
func (f *FakeClientProvider) SetClusterError(key types.NamespacedName, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.clients[key] = clientOrErr{err: err}
}
