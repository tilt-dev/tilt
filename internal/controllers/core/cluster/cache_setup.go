package cluster

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
)

// SetupCacheInvalidation configures the global cache invalidation system and returns a configured ConnectionManager
func SetupCacheInvalidation() *ConnectionManager {
	cm := NewConnectionManager()

	// Register the cache invalidation callback
	k8scontext.SetGlobalCacheInvalidator(func(clusterKey types.NamespacedName) {
		if cm != nil {
			cm.Delete(clusterKey)
		}
	})
	// Store globally for cross-package access (hack to avoid import cycles)
	SetGlobalConnectionManager(cm)

	return cm
}



// SetGlobalConnectionManager sets the global connection manager (hack for cross-package access)
func SetGlobalConnectionManager(cm *ConnectionManager) {
	// Set up the adapter for the k8s.ConnectionProvider interface
	adapter := &connectionProviderAdapter{cm: cm}
	k8s.SetGlobalConnectionProvider(adapter)
}

// connectionProviderAdapter adapts ConnectionManager to implement k8s.ConnectionProvider
type connectionProviderAdapter struct {
	cm *ConnectionManager
}

// GetK8sClient implements k8s.ConnectionProvider interface
func (a *connectionProviderAdapter) GetK8sClient(clusterKey types.NamespacedName) (k8s.Client, error) {
	client, _, err := a.cm.GetK8sClient(clusterKey)
	return client, err
}
