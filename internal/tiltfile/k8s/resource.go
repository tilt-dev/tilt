package k8s

import (
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

type KindInfo struct {
	ImageLocators    []k8s.ImageLocator
	PodReadinessMode model.PodReadinessMode
}

func InitialKinds() map[k8s.ObjectSelector]*KindInfo {
	sel, err := k8s.NewPartialMatchObjectSelector("batch/v1", "Job", "", "")
	if err != nil {
		panic(err)
	}
	return map[k8s.ObjectSelector]*KindInfo{
		sel: {PodReadinessMode: model.PodReadinessComplete},
	}
}
