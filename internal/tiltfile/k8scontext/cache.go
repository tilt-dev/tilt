package k8scontext

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// CacheInvalidationCallback is called when cache invalidation is needed
type CacheInvalidationCallback func(clusterKey types.NamespacedName)

// GlobalCacheInvalidator provides a way to register cache invalidation across the system
type GlobalCacheInvalidator struct {
	mu       sync.RWMutex
	callback CacheInvalidationCallback
}

var globalInvalidator = &GlobalCacheInvalidator{}

// SetGlobalCacheInvalidator sets the global cache invalidation callback
func SetGlobalCacheInvalidator(callback CacheInvalidationCallback) {
	globalInvalidator.mu.Lock()
	defer globalInvalidator.mu.Unlock()
	globalInvalidator.callback = callback
}

// InvalidateCache calls the global cache invalidation callback
func InvalidateCache(clusterKey types.NamespacedName) {
	globalInvalidator.mu.RLock()
	callback := globalInvalidator.callback
	globalInvalidator.mu.RUnlock()

	if callback != nil {
		callback(clusterKey)
	}
}
