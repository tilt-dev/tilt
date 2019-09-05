package k8s

import (
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/pkg/model"
)

// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
const ManagedByLabel = "app.kubernetes.io/managed-by"
const ManagedByValue = "tilt"

const TiltRunIDLabel = "tilt-runid"

var TiltRunID = uuid.New().String()

const ManifestNameLabel = "tilt-manifest"

func TiltRunLabel() model.LabelPair {
	return model.LabelPair{
		Key:   TiltRunIDLabel,
		Value: TiltRunID,
	}
}

func TiltRunSelector() labels.Selector {
	return labels.Set{TiltRunIDLabel: TiltRunID}.AsSelector()
}

func TiltManagedByLabel() model.LabelPair {
	return model.LabelPair{
		Key:   ManagedByLabel,
		Value: ManagedByValue,
	}
}

func NewTiltLabelMap() map[string]string {
	return map[string]string{
		ManagedByLabel: ManagedByValue,
		TiltRunIDLabel: TiltRunID,
	}
}

func LabelPairsToSelector(lps []model.LabelPair) labels.Selector {
	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}
	return ls.AsSelector()
}
