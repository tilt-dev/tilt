package cluster

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type FakeClientCache struct {
	mu      sync.Mutex
	clients map[types.NamespacedName]k8s.Client
	errors  map[types.NamespacedName]error
}

var _ ClientCache = &FakeClientCache{}

func NewFakeClientCache(defaultClient k8s.Client) *FakeClientCache {
	clients := make(map[types.NamespacedName]k8s.Client)
	if defaultClient != nil {
		defaultNN := types.NamespacedName{Name: v1alpha1.ClusterNameDefault}
		clients[defaultNN] = defaultClient
	}

	return &FakeClientCache{
		clients: clients,
		errors:  make(map[types.NamespacedName]error),
	}
}

func (f *FakeClientCache) GetK8sClient(key types.NamespacedName) (k8s.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err, ok := f.errors[key]; ok {
		return nil, err
	}

	cli, ok := f.clients[key]
	if !ok {
		return nil, NotFoundError
	}
	return cli, nil
}

func (f *FakeClientCache) SetK8sClient(key types.NamespacedName, client k8s.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients[key] = client
}

func (f *FakeClientCache) SetClusterError(key types.NamespacedName, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		delete(f.errors, key)
	} else {
		f.errors[key] = err
	}
}

func (f *FakeClientCache) AddK8sClient(key types.NamespacedName, client k8s.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.clients[key]; !ok {
		f.clients[key] = client
	}
}
