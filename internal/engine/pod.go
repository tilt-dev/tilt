package engine

import (
	"github.com/google/uuid"
	k8s "github.com/windmilleng/tilt/internal/k8s"
)

const TiltRunIDLabel = "tilt-runid"

var TiltRunID = uuid.New().String()

func TiltRunLabel() k8s.LabelPair {
	return k8s.LabelPair{
		Key:   TiltRunIDLabel,
		Value: TiltRunID,
	}
}
