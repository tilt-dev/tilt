package k8s

import (
	"net/url"
	"reflect"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type K8sEntity struct {
	Obj  runtime.Object
	Kind *schema.GroupVersionKind
}

func (e K8sEntity) Meta() metav1.ObjectMeta {
	objVal := reflect.ValueOf(e.Obj)
	if objVal.Kind() == reflect.Ptr {
		if objVal.IsNil() {
			return metav1.ObjectMeta{}
		}
		objVal = objVal.Elem()
	}

	if objVal.Kind() != reflect.Struct {
		return metav1.ObjectMeta{}
	}

	// Find a field with type ObjectMeta
	omType := reflect.TypeOf(metav1.ObjectMeta{})
	for i := 0; i < objVal.NumField(); i++ {
		fieldVal := objVal.Field(i)
		if omType != fieldVal.Type() {
			continue
		}

		if !fieldVal.CanInterface() {
			continue
		}

		metadata, ok := fieldVal.Interface().(metav1.ObjectMeta)
		if !ok {
			continue
		}

		return metadata
	}
	return metav1.ObjectMeta{}
}

func (e K8sEntity) Name() string {
	return e.Meta().Name
}

func (e K8sEntity) Namespace() Namespace {
	n := e.Meta().Namespace
	if n == "" {
		return DefaultNamespace
	}
	return Namespace(n)
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

func MutableEntities(entities []K8sEntity) []K8sEntity {
	result := make([]K8sEntity, 0)
	for _, e := range entities {
		if !e.ImmutableOnceCreated() {
			result = append(result, e)
		}
	}
	return result
}

type LoadBalancerSpec struct {
	Name      string
	Namespace Namespace
	Ports     []int32
}

type LoadBalancer struct {
	Spec LoadBalancerSpec
	URL  *url.URL
}

func ToLoadBalancerSpecs(entities []K8sEntity) []LoadBalancerSpec {
	result := make([]LoadBalancerSpec, 0)
	for _, e := range entities {
		lb, ok := ToLoadBalancerSpec(e)
		if ok {
			result = append(result, lb)
		}
	}
	return result
}

// Try to convert the current entity to a LoadBalancerSpec service
func ToLoadBalancerSpec(entity K8sEntity) (LoadBalancerSpec, bool) {
	service, ok := entity.Obj.(*v1.Service)
	if !ok {
		return LoadBalancerSpec{}, false
	}

	meta := service.ObjectMeta
	name := meta.Name
	spec := service.Spec
	if spec.Type != v1.ServiceTypeLoadBalancer {
		return LoadBalancerSpec{}, false
	}

	result := LoadBalancerSpec{
		Name:      name,
		Namespace: Namespace(meta.Namespace),
	}
	for _, portSpec := range spec.Ports {
		if portSpec.Port != 0 {
			result.Ports = append(result.Ports, portSpec.Port)
		}
	}

	if len(result.Ports) == 0 {
		return LoadBalancerSpec{}, false
	}

	return result, true
}

// PopMatching pops entities passing the given test, returning the popped
// entries and the remainder of the slice.
func PopEntities(entities []K8sEntity, test func(e K8sEntity) (bool, error)) (popped, rest []K8sEntity, err error) {
	for _, e := range entities {
		pass, err := test(e)
		if err != nil {
			return nil, nil, err
		}
		if pass {
			popped = append(popped, e.DeepCopy())
		} else {
			rest = append(rest, e.DeepCopy())
		}
	}
	return popped, rest, nil
}
