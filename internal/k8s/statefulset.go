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
// https://github.com/tilt-dev/tilt/issues/1962
//
// Tilt should change all statefulsets to use a parallel policy.
func InjectParallelPodManagementPolicy(entity K8sEntity) K8sEntity {
	entity = entity.DeepCopy()
	switch o := entity.Obj.(type) {
	case *v1.StatefulSet:
		o.Spec.PodManagementPolicy = v1.ParallelPodManagement
		return entity
	case *v1beta1.StatefulSet:
		o.Spec.PodManagementPolicy = v1beta1.ParallelPodManagement
		return entity
	case *v1beta2.StatefulSet:
		o.Spec.PodManagementPolicy = v1beta2.ParallelPodManagement
		return entity
	}
	return entity
}
