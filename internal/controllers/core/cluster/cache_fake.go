package cluster

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type FakeClientCache struct {
	*ConnectionManager
}

var _ ClientCache = &FakeClientCache{}

func NewFakeClientCache(defaultClient k8s.Client) *FakeClientCache {
	cm := NewConnectionManager()
	if defaultClient != nil {
		defaultNN := types.NamespacedName{Name: v1alpha1.ClusterNameDefault}
		cm.store(defaultNN, connection{connType: connectionTypeK8s, k8sClient: defaultClient})
	}

	return &FakeClientCache{
		ConnectionManager: cm,
	}
}

func (f *FakeClientCache) SetK8sClient(key types.NamespacedName, client k8s.Client) {
	f.store(key, connection{connType: connectionTypeK8s, k8sClient: client})
}

func (f *FakeClientCache) SetClusterError(key types.NamespacedName, err error) {
	errString := ""
	if err != nil {
		errString = err.Error()
	}
	f.store(key, connection{connType: connectionTypeK8s, error: errString})
}

func (f *FakeClientCache) AddK8sClient(key types.NamespacedName, client k8s.Client) {
	_, ok := f.load(key)
	if !ok {
		f.SetK8sClient(key, client)
	}
}
