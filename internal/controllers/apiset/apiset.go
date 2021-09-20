package apiset

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// A set of API Objects of different types.
type ObjectSet map[schema.GroupVersionResource]TypedObjectSet

func (s ObjectSet) GetOrCreateTypedSet(o Object) TypedObjectSet {
	gvk := o.GetGroupVersionResource()
	set := s[gvk]
	if set == nil {
		set = TypedObjectSet{}
		s[gvk] = set
	}
	return set
}

// A set of API Objects of the same type.
type TypedObjectSet map[string]Object

// An API object with the methods we need to do bulk creation.
type Object interface {
	ctrlclient.Object
	GetSpec() interface{}
	GetGroupVersionResource() schema.GroupVersionResource
	NewList() runtime.Object
}
