package k8s

import (
	"github.com/windmilleng/tilt/internal/model"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

// injectLabels injects the given labels into the given k8sEntity
// (if `overwrite`, replacing existing labels)
func injectLabels(entity K8sEntity, labels []model.LabelPair, overwrite bool) (K8sEntity, error) {
	entity = entity.DeepCopy()

	switch obj := entity.Obj.(type) {
	case *appsv1beta1.Deployment:
		allowLabelChangesInAppsDeploymentBeta1(obj)
	case *appsv1beta2.Deployment:
		allowLabelChangesInAppsDeploymentBeta2(obj)
	case *extv1beta1.Deployment:
		allowLabelChangesInExtDeploymentBeta1(obj)
	}

	metas, err := extractObjectMetas(&entity)
	if err != nil {
		return K8sEntity{}, err
	}

	for _, meta := range metas {
		if overwrite {
			meta.Labels = nil
		}
		for _, label := range labels {
			if meta.Labels == nil {
				meta.Labels = make(map[string]string, 1)
			}
			meta.Labels[label.Key] = label.Value
		}
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
func allowLabelChangesInAppsDeploymentBeta1(dep *appsv1beta1.Deployment) {
	selector := dep.Spec.Selector
	if selector != nil &&
		(len(selector.MatchLabels) > 0 || len(selector.MatchExpressions) > 0) {
		return
	}

	podSpecLabels := dep.Spec.Template.Labels
	matchLabels := make(map[string]string, len(podSpecLabels))
	for k, v := range podSpecLabels {
		matchLabels[k] = v
	}
	if dep.Spec.Selector == nil {
		dep.Spec.Selector = &metav1.LabelSelector{}
	}
	dep.Spec.Selector.MatchLabels = matchLabels
}

// see notes on allowLabelChangesInAppsDeploymentBeta1
func allowLabelChangesInAppsDeploymentBeta2(dep *appsv1beta2.Deployment) {
	selector := dep.Spec.Selector
	if selector != nil &&
		(len(selector.MatchLabels) > 0 || len(selector.MatchExpressions) > 0) {
		return
	}

	podSpecLabels := dep.Spec.Template.Labels
	matchLabels := make(map[string]string, len(podSpecLabels))
	for k, v := range podSpecLabels {
		matchLabels[k] = v
	}
	if dep.Spec.Selector == nil {
		dep.Spec.Selector = &metav1.LabelSelector{}
	}
	dep.Spec.Selector.MatchLabels = matchLabels
}

func allowLabelChangesInExtDeploymentBeta1(dep *extv1beta1.Deployment) {
	selector := dep.Spec.Selector
	if selector != nil &&
		(len(selector.MatchLabels) > 0 || len(selector.MatchExpressions) > 0) {
		return
	}

	podSpecLabels := dep.Spec.Template.Labels
	matchLabels := make(map[string]string, len(podSpecLabels))
	for k, v := range podSpecLabels {
		matchLabels[k] = v
	}
	if dep.Spec.Selector == nil {
		dep.Spec.Selector = &metav1.LabelSelector{}
	}
	dep.Spec.Selector.MatchLabels = matchLabels
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
	metas, err := extractObjectMetas(&e)
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
