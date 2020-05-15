package k8s

import (
	"reflect"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/pkg/model"
)

func makeLabelSet(lps []model.LabelPair) labels.Set {
	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}
	return ls
}

func makeLabelSelector(lps []model.LabelPair) string {
	return labels.SelectorFromSet(makeLabelSet(lps)).String()
}

func InjectLabels(entity K8sEntity, labels []model.LabelPair) (K8sEntity, error) {
	return injectLabels(entity, labels, false)
}

func OverwriteLabels(entity K8sEntity, labels []model.LabelPair) (K8sEntity, error) {
	return injectLabels(entity, labels, true)
}

// labels: labels to be added to `dest`
// overwrite: if true, merge `labels` into `dest`. if false, replace `dest` with `labels`
// addNew: if true, add all `labels` to `dest`. if false, only add `labels` whose keys are already in `dest`
func applyLabelsToMap(dest *map[string]string, labels []model.LabelPair, overwrite bool, addNew bool) {
	orig := *dest
	if overwrite {
		*dest = nil
	}
	for _, label := range labels {
		if *dest == nil {
			*dest = make(map[string]string, 1)
		}
		preexisting := false
		if orig != nil {
			_, preexisting = orig[label.Key]
		}

		if addNew || preexisting {
			(*dest)[label.Key] = label.Value
		}
	}
}

// injectLabels injects the given labels into the given k8sEntity
// (if `overwrite`, replacing existing labels)
func injectLabels(entity K8sEntity, labels []model.LabelPair, overwrite bool) (K8sEntity, error) {
	entity = entity.DeepCopy()

	// Don't modify persistent volume claims
	// because they're supposed to be immutable.
	pvc := reflect.TypeOf(v1.PersistentVolumeClaim{})
	metas, err := extractObjectMetas(&entity, func(v reflect.Value) bool {
		return v.Type() != pvc
	})
	if err != nil {
		return K8sEntity{}, err
	}

	for _, meta := range metas {
		applyLabelsToMap(&meta.Labels, labels, overwrite, true)
	}

	switch obj := entity.Obj.(type) {
	case *appsv1beta1.Deployment,
		*appsv1beta2.Deployment,
		*extv1beta1.Deployment,
		*appsv1beta2.ReplicaSet,
		*extv1beta1.ReplicaSet,
		*appsv1beta2.DaemonSet,
		*extv1beta1.DaemonSet,
		*appsv1beta1.StatefulSet,
		*appsv1beta2.StatefulSet:
		allowLabelChangesInOptionalSelector(obj)
	}

	selectors, err := extractSelectors(&entity, func(v reflect.Value) bool {
		return v.Type() != pvc
	})
	if err != nil {
		return K8sEntity{}, err
	}
	for _, selector := range selectors {
		applyLabelsToMap(&selector.MatchLabels, labels, overwrite, false)
	}

	// ServiceSpecs have a map[string]string instead of a LabelSelector, so handle them specially
	serviceSpecs, err := extractServiceSpecs(&entity)
	if err != nil {
		return K8sEntity{}, err
	}
	for _, s := range serviceSpecs {
		applyLabelsToMap(&s.Selector, labels, overwrite, false)
	}
	return entity, nil
}

// In the v1beta1 API, if a Deployment didn't have a selector,
// Kubernetes would automatically infer a selector based on the labels
// in the pod.
//
// The problem is that Selectors are immutable. So this had the unintended
// side-effect of making pod labels immutable. Since many tools attach
// arbitrary labels to pods, v1beta1 Deployments broke lots of tools.
//
// The v1 Deployment fixed this problem by making Selector mandatory.
// But for old versions of Deployment, we need to auto-infer the selector
// before we add labels to the pod.
func allowLabelChangesInOptionalSelector(obj runtime.Object) {
	// First, check to make sure this has a Spec object with:
	// 1) A Selector field of type metav1.LabelSelector
	// 2) A Template field of type v1.PodTemplateSpec
	objPtrV := reflect.ValueOf(obj)
	if objPtrV.Kind() != reflect.Ptr {
		return
	}

	objV := objPtrV.Elem()
	if objV.Kind() != reflect.Struct {
		return
	}

	specField := objV.FieldByName("Spec")
	if specField.Kind() != reflect.Struct {
		return
	}

	specType := specField.Type()
	selectorFieldType, ok := specType.FieldByName("Selector")
	if !ok || selectorFieldType.Type != reflect.TypeOf(&metav1.LabelSelector{}) {
		return
	}

	podTemplateFieldType, ok := specType.FieldByName("Template")
	if !ok || podTemplateFieldType.Type != reflect.TypeOf(v1.PodTemplateSpec{}) {
		return
	}

	selectorField := specField.FieldByName("Selector")
	var selector *metav1.LabelSelector
	if selectorField.IsValid() {
		selector, ok = selectorField.Interface().(*metav1.LabelSelector)
		if !ok {
			return
		}
	}

	// If the selector is already filled in, we don't need to do anything.
	if selector != nil &&
		(len(selector.MatchLabels) > 0 || len(selector.MatchExpressions) > 0) {
		return
	}

	podTemplateField := specField.FieldByName("Template")
	podTemplate, ok := podTemplateField.Interface().(v1.PodTemplateSpec)
	if !ok {
		return
	}

	podSpecLabels := podTemplate.Labels
	matchLabels := make(map[string]string, len(podSpecLabels))
	for k, v := range podSpecLabels {
		matchLabels[k] = v
	}

	if selector == nil {
		selector = &metav1.LabelSelector{}
	}
	selector.MatchLabels = matchLabels
	selectorField.Set(reflect.ValueOf(selector))
}

// SelectorMatchesLabels indicates whether the pod selector of the given entity matches the given label(s).
// Currently only supports Services, but may be expanded to support other types that
// match pods via selectors.
func (e K8sEntity) SelectorMatchesLabels(labels map[string]string) bool {
	svc, ok := e.Obj.(*v1.Service)
	if !ok {
		return false
	}
	selector := svc.Spec.Selector
	for k, selVal := range selector {
		realVal, ok := labels[k]
		if !ok || realVal != selVal {
			return false
		}
	}
	return true

}

// MatchesMetadataLabels indicates whether the given label(s) are a subset
// of metadata labels for the given entity.
func (e K8sEntity) MatchesMetadataLabels(labels map[string]string) (bool, error) {
	// TODO(nick): This doesn't seem right.
	// This should only look at the top-level obj meta, not all of them.
	metas, err := extractObjectMetas(&e, NoFilter)
	if err != nil {
		return false, err
	}

	for _, meta := range metas {
		for k, v := range labels {
			realVal, ok := meta.Labels[k]
			if !ok || realVal != v {
				return false, nil
			}
		}
	}

	return true, nil

}
