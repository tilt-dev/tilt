package engine

import (
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
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

// Extract image targets iff they can be updated in-place in a container.
func extractImageTargetsForLiveUpdates(specs []model.TargetSpec, stateSet store.BuildStateSet) ([]model.ImageTarget, error) {
	iTargets := make([]model.ImageTarget, 0)
	for _, spec := range specs {
		iTarget, ok := spec.(model.ImageTarget)
		if !ok {
			continue
		}

		state := stateSet[iTarget.ID()]
		if state.IsEmpty() {
			return nil, SilentRedirectToNextBuilderf("In-place build does not support initial deploy")
		}

		// If this image doesn't need to be built at all, we can skip it.
		if !state.NeedsImageBuild() {
			continue
		}

		fbInfo := iTarget.AnyFastBuildInfo()
		luInfo := iTarget.AnyLiveUpdateInfo()
		if fbInfo.Empty() && luInfo.Empty() {
			return nil, SilentRedirectToNextBuilderf("In-place build requires either FastBuild or LiveUpdate")
		}

		// Now that we have fast build information, we know this CAN be updated in
		// a container. Check to see if we have enough information about the container
		// that would need to be updated.
		deployInfo := state.DeployInfo
		if deployInfo.Empty() {
			return nil, RedirectToNextBuilderInfof("don't have info for deployed container (often a result of the deployment not yet being ready)")
		}
		iTargets = append(iTargets, iTarget)
	}
	return iTargets, nil
}

// Returns true if the given image is deployed to one of the given k8s targets.
// Note that some images are injected into other images, so may never be deployed.
func isImageDeployedToK8s(iTarget model.ImageTarget, kTargets []model.K8sTarget) bool {
	id := iTarget.ID()
	for _, kTarget := range kTargets {
		for _, depID := range kTarget.DependencyIDs() {
			if depID == id {
				return true
			}
		}
	}
	return false
}

// Returns true if the given image is deployed to one of the given docker-compose targets.
// Note that some images are injected into other images, so may never be deployed.
func isImageDeployedToDC(iTarget model.ImageTarget, dcTarget model.DockerComposeTarget) bool {
	id := iTarget.ID()
	for _, depID := range dcTarget.DependencyIDs() {
		if depID == id {
			return true
		}
	}
	return false
}
