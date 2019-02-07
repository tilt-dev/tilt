package k8s

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/windmilleng/tilt/internal/model"
	"k8s.io/apimachinery/pkg/labels"
)

const TiltRunIDLabel = "tilt-runid"

var TiltRunID = uuid.New().String()

const ManifestNameLabel = "tilt-manifest"

const TiltDeployIDLabel = "tilt-deployid"

type DeployID int64 // Unix ns after epoch -- uniquely identify a deploy

func TiltRunLabel() model.LabelPair {
	return model.LabelPair{
		Key:   TiltRunIDLabel,
		Value: TiltRunID,
	}
}

func NewDeployID() DeployID {
	return DeployID(time.Now().UnixNano())
}

func TiltDeployLabel(dID DeployID) model.LabelPair {
	return model.LabelPair{
		Key:   TiltDeployIDLabel,
		Value: strconv.Itoa(int(dID)),
	}
}

func TiltRunSelector() labels.Selector {
	return labels.Set{TiltRunIDLabel: TiltRunID}.AsSelector()
}
