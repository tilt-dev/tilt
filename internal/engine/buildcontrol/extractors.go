package buildcontrol

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

// Extract the targets that we can apply, or nil if we can't apply these targets.
func extractImageAndK8sTargets(specs []model.TargetSpec) (iTargets []model.ImageTarget, kTargets []model.K8sTarget) {
	for _, s := range specs {
		switch s := s.(type) {
		case model.ImageTarget:
			iTargets = append(iTargets, s)
		case model.K8sTarget:
			kTargets = append(kTargets, s)
		default:
			return nil, nil
		}
	}
	return iTargets, kTargets
}
