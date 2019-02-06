package engine

import (
	"github.com/google/uuid"
	"github.com/windmilleng/tilt/internal/model"
	"k8s.io/apimachinery/pkg/labels"
)

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
