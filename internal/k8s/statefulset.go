package k8s

import (
	v1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/apps/v1beta2"
)

// By default, StatefulSets use OrderedPodManagement.
//
// This is a bad policy for development. If the pod goes into a crash loop,
// the StatefulSet operator will get wedged and require manual intervention.
// See:
// https://github.com/windmilleng/tilt/issues/1962
//
// Tilt should change all statefulsets to use a parallel policy.
func InjectParallelPodManagementPolicy(entity K8sEntity) K8sEntity {
	switch entity.Obj.(type) {
	case *v1.StatefulSet:
		entity = entity.DeepCopy()
		entity.Obj.(*v1.StatefulSet).Spec.PodManagementPolicy = v1.ParallelPodManagement
		return entity
	case *v1beta1.StatefulSet:
		entity = entity.DeepCopy()
		entity.Obj.(*v1beta1.StatefulSet).Spec.PodManagementPolicy = v1beta1.ParallelPodManagement
		return entity
	case *v1beta2.StatefulSet:
		entity = entity.DeepCopy()
		entity.Obj.(*v1beta2.StatefulSet).Spec.PodManagementPolicy = v1beta2.ParallelPodManagement
		return entity
	}
	return entity
}
