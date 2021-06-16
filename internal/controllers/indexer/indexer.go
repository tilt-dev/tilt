package indexer

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// A key to help index objects we watch.
type Key struct {
	Name types.NamespacedName
	GVK  schema.GroupVersionKind
}

type KeyFunc func(obj client.Object) []Key

// Helper struct to help reconcilers determine when
// to start their objects when a dependency triggers.
type Indexer struct {
	scheme *runtime.Scheme

	indexFunc KeyFunc

	// A map to help determine which Objects to reconcile when one of the objects
	// they're watching change.
	//
	// The first key is the name and type of the object being watched.
	//
	// The second key is the name of the main object being reconciled.
	//
	// For example, if a Cmd is triggered by a FileWatch, the first
	// key is the FileWatch name and GVK, while the second key is the Cmd name.
	indexByWatchedObjects map[Key]map[types.NamespacedName]bool

	mu sync.Mutex
}

func NewIndexer(scheme *runtime.Scheme, f KeyFunc) *Indexer {
	return &Indexer{
		scheme:                scheme,
		indexFunc:             f,
		indexByWatchedObjects: make(map[Key]map[types.NamespacedName]bool),
	}
}

// Register the watched object for the given primary object.
func (m *Indexer) OnReconcile(name types.NamespacedName, obj client.Object) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Delete all the mappings for this object.
	for _, index := range m.indexByWatchedObjects {
		delete(index, name)
	}

	// Re-add all the mappings.
	for _, key := range m.indexFunc(obj) {
		index, ok := m.indexByWatchedObjects[key]
		if !ok {
			index = make(map[types.NamespacedName]bool)
			m.indexByWatchedObjects[key] = index
		}

		index[name] = true
	}
}

// Given an update of a watched object, return the names of objects watching it
// that we need to reconcile.
func (m *Indexer) Enqueue(obj client.Object) []reconcile.Request {
	gvk, err := apiutil.GVKForObject(obj, m.scheme)
	if err != nil {
		panic(fmt.Sprintf("Unrecognized object: %v", err))
	}
	key := Key{
		Name: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()},
		GVK:  gvk,
	}
	return m.EnqueueKey(key)
}

// Enqueue() when we don't have the full object, only the name and kind.
func (m *Indexer) EnqueueKey(key Key) []reconcile.Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]reconcile.Request, 0, len(m.indexByWatchedObjects[key]))
	for watchingName := range m.indexByWatchedObjects[key] {
		result = append(result, reconcile.Request{NamespacedName: watchingName})
	}
	return result
}
