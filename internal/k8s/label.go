package k8s

import (
	"k8s.io/api/core/v1"
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
