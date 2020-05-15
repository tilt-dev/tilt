package k8s

import (
	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/tilt/pkg/model"
)

// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
const ManagedByLabel = "app.kubernetes.io/managed-by"
const ManagedByValue = "tilt"

const ManifestNameLabel = "tilt-manifest"

func TiltManagedByLabel() model.LabelPair {
	return model.LabelPair{
		Key:   ManagedByLabel,
		Value: ManagedByValue,
	}
}

func ManagedByTiltSelector() labels.Selector {
	return labels.Set{ManagedByLabel: ManagedByValue}.AsSelector()
}

func NewTiltLabelMap() map[string]string {
	return map[string]string{
		ManagedByLabel: ManagedByValue,
	}
}

func LabelPairsToSelector(lps []model.LabelPair) labels.Selector {
	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}
	return ls.AsSelector()
}
