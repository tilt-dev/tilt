package k8s

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/tilt-dev/tilt/internal/kustomize"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/internal/container"
)

type K8sEntity struct {
	Obj runtime.Object
}

func NewK8sEntity(obj runtime.Object) K8sEntity {
	return K8sEntity{Obj: obj}
}

type entityList []K8sEntity

func (l entityList) Len() int { return len(l) }
func (l entityList) Less(i, j int) bool {
	// Sort entities by the priority of their Kind
	indexI := kustomize.TypeOrders[l[i].GVK().Kind]
	indexJ := kustomize.TypeOrders[l[j].GVK().Kind]
	if indexI != indexJ {
		return indexI < indexJ
	}
	return i < j
}
func (l entityList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func SortedEntities(entities []K8sEntity) []K8sEntity {
	entList := entityList(CopyEntities(entities))
	sort.Stable(entList)
	return []K8sEntity(entList)
}

func ReverseSortedEntities(entities []K8sEntity) []K8sEntity {
	entList := entityList(CopyEntities(entities))
	sort.Sort(sort.Reverse(entList))
	return entList
}

func (e K8sEntity) Meta() metav1.Object {
	m, err := meta.Accessor(e.Obj)
	if err != nil {
		return &metav1.ObjectMeta{}
	}
	return m
}

func (e K8sEntity) ToObjectReference() v1.ObjectReference {
	meta := e.Meta()
	apiVersion, kind := e.GVK().ToAPIVersionAndKind()
	return v1.ObjectReference{
		Kind:       kind,
		APIVersion: apiVersion,
		Name:       meta.GetName(),
		Namespace:  meta.GetNamespace(),
		UID:        meta.GetUID(),
	}
}

func (e K8sEntity) WithNamespace(ns string) K8sEntity {
	newE := e.DeepCopy()
	newE.Meta().SetNamespace(ns)
	return newE
}

func (e K8sEntity) GVK() schema.GroupVersionKind {
	gvk := e.Obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		// On typed go objects, the GVK is usually empty by convention, so we grab it from the Scheme
		// See https://github.com/kubernetes/kubernetes/pull/59264#issuecomment-362575608
		// for discussion on why the API behaves this way.
		gvks, _, _ := scheme.Scheme.ObjectKinds(e.Obj)
		if len(gvks) > 0 {
			return gvks[0]
		}
	}
	return gvk
}

// Clean up internal bookkeeping fields. See
// https://github.com/kubernetes/kubernetes/issues/90066
func (e K8sEntity) Clean() {
	e.Meta().SetManagedFields(nil)

	annotations := e.Meta().GetAnnotations()
	if len(annotations) != 0 {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	}
}

func (e K8sEntity) SetUID(uid string) {
	e.Meta().SetUID(types.UID(uid))
}

func (e K8sEntity) Name() string {
	return e.Meta().GetName()
}

func (e K8sEntity) Namespace() Namespace {
	n := e.Meta().GetNamespace()
	if n == "" {
		return DefaultNamespace
	}
	return Namespace(n)
}

func (e K8sEntity) NamespaceOrDefault(defaultVal string) string {
	n := e.Meta().GetNamespace()
	if n == "" {
		return defaultVal
	}
	return n
}

func (e K8sEntity) UID() types.UID {
	return e.Meta().GetUID()
}

func (e K8sEntity) Annotations() map[string]string {
	return e.Meta().GetAnnotations()
}

func (e K8sEntity) Labels() map[string]string {
	return e.Meta().GetLabels()
}

// Most entities can be updated once running, but a few cannot.
func (e K8sEntity) ImmutableOnceCreated() bool {
	return e.GVK().Kind == "Job" || e.GVK().Kind == "Pod"
}

func (e K8sEntity) DeepCopy() K8sEntity {
	return NewK8sEntity(e.Obj.DeepCopyObject())
}

func CopyEntities(entities []K8sEntity) []K8sEntity {
	res := make([]K8sEntity, len(entities))
	for i, e := range entities {
		res[i] = e.DeepCopy()
	}
	return res
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

func FilterByImage(entities []K8sEntity, img container.RefSelector, locators []ImageLocator, inEnvVars bool) (passing, rest []K8sEntity, err error) {
	return Filter(entities, func(e K8sEntity) (bool, error) { return e.HasImage(img, locators, inEnvVars) })
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

	// Get the namespace of the workload - only match Services in the same namespace
	workloadNamespace := withPodSpec.Namespace()

	var allMatches []K8sEntity
	remaining := append([]K8sEntity{}, entities...)
	for _, template := range podTemplates {
		// Filter by both: selector matches labels AND same namespace
		match, rest, err := filterBySelectorMatchesLabelsAndNamespace(remaining, template.Labels, workloadNamespace)
		if err != nil {
			return nil, nil, errors.Wrap(err, "filtering entities by label and namespace")
		}
		allMatches = append(allMatches, match...)
		remaining = rest
	}
	return allMatches, remaining, nil
}

// filterBySelectorMatchesLabelsAndNamespace filters entities that match both:
// 1. The selector matches the given labels
// 2. The entity is in the same namespace as the workload (or has no explicit namespace)
// This fixes https://github.com/tilt-dev/tilt/issues/6311
func filterBySelectorMatchesLabelsAndNamespace(entities []K8sEntity, labels map[string]string, workloadNamespace Namespace) (passing, rest []K8sEntity, err error) {
	return Filter(entities, func(e K8sEntity) (bool, error) {
		// Must match selector labels
		if !e.SelectorMatchesLabels(labels) {
			return false, nil
		}
		// Check namespace matching:
		// - If entity has no explicit namespace (uses default), allow matching any workload
		//   (maintains backward compatibility)
		// - If entity has an explicit namespace, it must match the workload's namespace
		entityNamespace := e.Namespace()
		if entityNamespace == DefaultNamespace {
			// No explicit namespace on entity, allow matching
			return true, nil
		}
		// Entity has explicit namespace, must match workload's namespace
		return entityNamespace == workloadNamespace, nil
	})
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
	return strings.EqualFold(e.GVK().Kind, kind)
}

func NewNamespaceEntity(name string) K8sEntity {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, name)
	entities, err := ParseYAMLFromString(yaml)

	// Something is wrong with our format string; this is definitely on us
	if err != nil {
		panic(fmt.Sprintf("unexpected error making new namespace: %v", err))
	} else if len(entities) != 1 {
		// Something is wrong with our format string; this is definitely on us
		panic(fmt.Sprintf(
			"unexpected error making new namespace: got %d entities, expected exactly one", len(entities)))
	}
	return entities[0]
}
