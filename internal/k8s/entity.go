package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type K8sEntity struct {
	Obj  runtime.Object
	Kind *schema.GroupVersionKind
}

// Most entities can be updated once running, but a few cannot.
func (e K8sEntity) ImmutableOnceCreated() bool {
	if e.Kind != nil {
		// TODO(nick): Add more entities.
		return e.Kind.Kind == "Job"
	}
	return false
}

func (e K8sEntity) DeepCopy() K8sEntity {
	// GroupVersionKind is a struct of string values, so dereferencing the pointer
	// is an adequate copy.
	kind := *e.Kind
	return K8sEntity{
		Obj:  e.Obj.DeepCopyObject(),
		Kind: &kind,
	}
}

func ImmutableEntities(entities []K8sEntity) []K8sEntity {
	result := make([]K8sEntity, 0)
	for _, e := range entities {
		if e.ImmutableOnceCreated() {
			result = append(result, e)
		}
	}
	return result
}

type LoadBalancer struct {
	Name  string
	Ports []int32
}

func ToLoadBalancers(entities []K8sEntity) []LoadBalancer {
	result := make([]LoadBalancer, 0)
	for _, e := range entities {
		lb, ok := ToLoadBalancer(e)
		if ok {
			result = append(result, lb)
		}
	}
	return result
}

// Try to convert the current entity to a LoadBalancer service
func ToLoadBalancer(entity K8sEntity) (LoadBalancer, bool) {
	service, ok := entity.Obj.(*v1.Service)
	if !ok {
		return LoadBalancer{}, false
	}

	meta := service.ObjectMeta
	name := meta.Name
	spec := service.Spec
	if spec.Type != v1.ServiceTypeLoadBalancer {
		return LoadBalancer{}, false
	}

	result := LoadBalancer{Name: name}
	for _, portSpec := range spec.Ports {
		if portSpec.Port != 0 {
			result.Ports = append(result.Ports, portSpec.Port)
		}
	}

	if len(result.Ports) == 0 {
		return LoadBalancer{}, false
	}

	return result, true
}

// Get Deployment name
func GetDeploymentName(entity K8sEntity) string {
	service, ok := entity.Obj.(*v1.Service)
	if !ok {
		return ""
	}

	meta := service.ObjectMeta
	return meta.Name
}
