package k8s

import (
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type LabelPair struct {
	Key   string
	Value string
}

func makeLabelSet(lps []LabelPair) labels.Set {
	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}
	return ls
}

func makeLabelSelector(lps []LabelPair) string {
	return labels.SelectorFromSet(makeLabelSet(lps)).String()
}

func InjectLabels(entity K8sEntity, labels []LabelPair) (K8sEntity, error) {
	entity = entity.DeepCopy()

	switch obj := entity.Obj.(type) {
	case *v1beta1.Deployment:
		allowLabelChangesInDeploymentBeta1(obj)
	case *v1beta2.Deployment:
		allowLabelChangesInDeploymentBeta2(obj)
	}

	metas, err := extractObjectMetas(&entity)
	if err != nil {
		return K8sEntity{}, err
	}

	for _, meta := range metas {
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
func allowLabelChangesInDeploymentBeta1(dep *v1beta1.Deployment) {
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

// see notes on allowLabelChangesInDeploymentBeta1
func allowLabelChangesInDeploymentBeta2(dep *v1beta2.Deployment) {
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

// MatchesLevel indicates whether the selector of the given entity matches the given label(s).
// Currently only supports Services, but may be expanded to support other types that
// match pods via selectors.
func (e K8sEntity) MatchesLabels(labels map[string]string) bool {
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
