package k8s

import (
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

type KindInfo struct {
	ImageLocators    []k8s.ImageLocator
	PodReadinessMode model.PodReadinessMode
}
