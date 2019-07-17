package k8s

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/windmilleng/tilt/internal/container"
)

type K8sEntity struct {
	Obj  runtime.Object
	Kind *schema.GroupVersionKind
}

type k8sMeta interface {
	GetName() string
	GetNamespace() string
	GetUID() types.UID
	GetLabels() map[string]string
}

type emptyMeta struct{}

func (emptyMeta) GetName() string              { return "" }
func (emptyMeta) GetNamespace() string         { return "" }
func (emptyMeta) GetUID() types.UID            { return "" }
func (emptyMeta) GetLabels() map[string]string { return make(map[string]string) }

var _ k8sMeta = emptyMeta{}
var _ k8sMeta = &metav1.ObjectMeta{}

func (e K8sEntity) meta() k8sMeta {
	if unstruct := e.maybeUnstructuredMeta(); unstruct != nil {
		return unstruct
	}

	if structured, _ := e.maybeStructuredMeta(); structured != nil {
		return structured
	}

	return emptyMeta{}
}

func (e K8sEntity) maybeUnstructuredMeta() *unstructured.Unstructured {
	unstruct, isUnstructured := e.Obj.(*unstructured.Unstructured)
	if isUnstructured {
		return unstruct
	}
	return nil
}

func (e K8sEntity) maybeStructuredMeta() (meta *metav1.ObjectMeta, fieldIndex int) {
	objVal := reflect.ValueOf(e.Obj)
	if objVal.Kind() == reflect.Ptr {
		if objVal.IsNil() {
			return nil, -1
		}
		objVal = objVal.Elem()
	}

	if objVal.Kind() != reflect.Struct {
		return nil, -1
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

		return &metadata, i
	}
	return nil, -1
}

func SetUIDForTest(t *testing.T, e *K8sEntity, UID string) {
	unstruct := e.maybeUnstructuredMeta()
	if unstruct != nil {
		t.Fatal("SetUIDForTesting not yet implemented for unstructured metadata")
	}

	structured, i := e.maybeStructuredMeta()
	if structured == nil {
		t.Fatalf("Cannot set UID -- entity has neither unstructured nor structured metadata. k8s entity: %+v", e)
	}

	structured.SetUID(types.UID(UID))
	objVal := reflect.ValueOf(e.Obj)
	if objVal.Kind() == reflect.Ptr {
		if objVal.IsNil() {
			t.Fatalf("Cannot set UID -- e.Obj is a pointer. k8s entity: %+v", e)
		}
		objVal = objVal.Elem()
	}

	fieldVal := objVal.Field(i)
	metaVal := reflect.ValueOf(*structured)
	fieldVal.Set(metaVal)
}

func (e K8sEntity) Name() string {
	return e.meta().GetName()
}

func (e K8sEntity) Namespace() Namespace {
	n := e.meta().GetNamespace()
	if n == "" {
		return DefaultNamespace
	}
	return Namespace(n)
}

func (e K8sEntity) UID() UID {
	return UID(e.meta().GetUID())
}

func (e K8sEntity) Labels() map[string]string {
	return e.meta().GetLabels()
}

// Most entities can be updated once running, but a few cannot.
func (e K8sEntity) ImmutableOnceCreated() bool {
	if e.Kind != nil {
		return e.Kind.Kind == "Job" || e.Kind.Kind == "Pod"
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

// EntitiesWithDependentsAndRest returns two lists of k8s entities: those that may have dependencies,
// which we will therefore want to apply first (i.e. namespaces and CRDs -- e.g. trying to create a
// pod in a nonexistent namespace causes an error); and the rest of the entities.
func EntitiesWithDependentsAndRest(entities []K8sEntity) (withDependents, rest []K8sEntity) {
	var ns []K8sEntity
	var crd []K8sEntity

	for _, e := range entities {
		if e.Kind.Kind == "Namespace" {
			ns = append(ns, e)
		} else if e.Kind.Kind == "CustomResourceDefinition" {
			crd = append(crd, e)
		} else {
			rest = append(rest, e)
		}
	}

	return append(ns, crd...), rest
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

// Filter returns two slices of entities: those passing the given test, and the remainder of the input.
func Filter(entities []K8sEntity, test func(e K8sEntity) (bool, error)) (passing, rest []K8sEntity, err error) {
	for _, e := range entities {
		pass, err := test(e)
		if err != nil {
			return nil, nil, err
		}
		if pass {
			passing = append(passing, e)
		} else {
			rest = append(rest, e)
		}
	}
	return passing, rest, nil
}

func FilterByImage(entities []K8sEntity, img container.RefSelector, imageJSONPaths func(K8sEntity) []JSONPath, inEnvVars bool) (passing, rest []K8sEntity, err error) {
	return Filter(entities, func(e K8sEntity) (bool, error) { return e.HasImage(img, imageJSONPaths(e), inEnvVars) })
}

func FilterBySelectorMatchesLabels(entities []K8sEntity, labels map[string]string) (passing, rest []K8sEntity, err error) {
	return Filter(entities, func(e K8sEntity) (bool, error) { return e.SelectorMatchesLabels(labels), nil })
}

func FilterByMetadataLabels(entities []K8sEntity, labels map[string]string) (passing, rest []K8sEntity, err error) {
	return Filter(entities, func(e K8sEntity) (bool, error) { return e.MatchesMetadataLabels(labels) })
}

func FilterByHasPodTemplateSpec(entities []K8sEntity) (passing, rest []K8sEntity, err error) {
	return Filter(entities, func(e K8sEntity) (bool, error) {
		templateSpecs, err := ExtractPodTemplateSpec(&e)
		if err != nil {
			return false, err
		}
		return len(templateSpecs) > 0, nil
	})
}

func FilterByMatchesPodTemplateSpec(withPodSpec K8sEntity, entities []K8sEntity) (passing, rest []K8sEntity, err error) {
	podTemplates, err := ExtractPodTemplateSpec(withPodSpec)
	if err != nil {
		return nil, nil, errors.Wrap(err, "extracting pod template spec")
	}

	if len(podTemplates) == 0 {
		return nil, entities, nil
	}

	var allMatches []K8sEntity
	remaining := append([]K8sEntity{}, entities...)
	for _, template := range podTemplates {
		match, rest, err := FilterBySelectorMatchesLabels(remaining, template.Labels)
		if err != nil {
			return nil, nil, errors.Wrap(err, "filtering entities by label")
		}
		allMatches = append(allMatches, match...)
		remaining = rest
	}
	return allMatches, remaining, nil
}

func (e K8sEntity) HasName(name string) bool {
	return e.Name() == name
}

func (e K8sEntity) HasNamespace(ns string) bool {
	realNs := e.Namespace()
	if ns == "" {
		return realNs == DefaultNamespace
	}
	return realNs.String() == ns
}

func (e K8sEntity) HasKind(kind string) bool {
	// TODO(maia): support kind aliases (e.g. "po" for "pod")
	return strings.ToLower(e.Kind.Kind) == strings.ToLower(kind)
}
