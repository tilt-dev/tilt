package k8s

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ObjRefList []v1.ObjectReference

func (o ObjRefList) ContainsUID(uid types.UID) bool {
	_, ok := o.GetByUID(uid)
	return ok
}

func (o ObjRefList) GetByUID(uid types.UID) (v1.ObjectReference, bool) {
	for _, ref := range o {
		if ref.UID == uid {
			return ref, true
		}
	}
	return v1.ObjectReference{}, false
}

func (o ObjRefList) UIDSet() UIDSet {
	out := NewUIDSet()
	for _, ref := range o {
		out.Add(ref.UID)
	}
	return out
}

func ToRefList(entities []K8sEntity) ObjRefList {
	refs := make(ObjRefList, len(entities))
	for i, entity := range entities {
		refs[i] = entity.ToObjectReference()
	}
	return refs
}
