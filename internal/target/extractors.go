package target

import (
	"fmt"

	"github.com/pkg/errors"

	errors2 "github.com/windmilleng/tilt/internal/engine/errors"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
)

// Extract the targets that we can apply, or nil if we can't apply these targets.
func ExtractImageAndK8sTargets(specs []model.TargetSpec) (iTargets []model.ImageTarget, kTargets []model.K8sTarget) {
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

// If there are images that can be updated in-place in a container, return
// a state tree of what needs to be updated.
func ExtractImageTargetsForLiveUpdates(specs []model.TargetSpec, stateSet store.BuildStateSet) ([]LiveUpdateStateTree, error) {
	g, err := model.NewTargetGraph(specs)
	if err != nil {
		return nil, errors.Wrap(err, "ExtractImageTargetsForLiveUpdates")
	}

	if !g.IsSingleSourceDAG() {
		return nil, fmt.Errorf("Cannot extract live updates on this build graph structure")
	}

	result := make([]LiveUpdateStateTree, 0)

	deployedImages := g.DeployedImages()
	for _, iTarget := range deployedImages {
		state := stateSet[iTarget.ID()]
		if state.IsEmpty() {
			return nil, errors2.SilentRedirectToNextBuilderf("In-place build does not support initial deploy")
		}

		hasFileChangesIDs, err := hasFileChangesTree(g, iTarget, stateSet)
		if err != nil {
			return nil, errors.Wrap(err, "ExtractImageTargetsForLiveUpdates")
		}

		// If this image and none of its dependencies need a rebuild,
		// we can skip it.
		if len(hasFileChangesIDs) == 0 {
			continue
		}

		fbInfo := iTarget.AnyFastBuildInfo()
		luInfo := iTarget.AnyLiveUpdateInfo()
		if fbInfo.Empty() && luInfo.Empty() {
			return nil, errors2.SilentRedirectToNextBuilderf("In-place build requires either FastBuild or LiveUpdate")
		}

		// Now that we have fast build information, we know this CAN be updated in
		// a container. Check to see if we have enough information about the container
		// that would need to be updated.
		deployInfo := state.DeployInfo
		if deployInfo.Empty() {
			return nil, errors2.RedirectToNextBuilderInfof("don't have info for deployed container of image %q (often a result of the deployment not yet being ready)", iTarget.DeploymentRef.String())
		}

		filesChanged, err := filesChangedTree(g, iTarget, stateSet)
		if err != nil {
			return nil, errors.Wrap(err, "ExtractImageTargetsForLiveUpdates")
		}

		result = append(result, LiveUpdateStateTree{
			ITarget:           iTarget,
			FilesChanged:      filesChanged,
			ITargetState:      state,
			HasFileChangesIDs: hasFileChangesIDs,
		})
	}

	return result, nil
}

// Returns true if the given image is deployed to one of the given k8s targets.
// Note that some images are injected into other images, so may never be deployed.
func IsImageDeployedToK8s(iTarget model.ImageTarget, kTargets []model.K8sTarget) bool {
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

func AllImagesDeployerToK8s(specs []model.TargetSpec) bool {
	iTargets, kTargets := ExtractImageAndK8sTargets(specs)
	for _, iTarg := range iTargets {
		if !IsImageDeployedToK8s(iTarg, kTargets) {
			return false
		}
	}
	return true
}

// Returns true if the given image is deployed to one of the given docker-compose targets.
// Note that some images are injected into other images, so may never be deployed.
func IsImageDeployedToDC(iTarget model.ImageTarget, dcTarget model.DockerComposeTarget) bool {
	id := iTarget.ID()
	for _, depID := range dcTarget.DependencyIDs() {
		if depID == id {
			return true
		}
	}
	return false
}

// Given a target, return all the target IDs in its tree of dependencies that
// have changed files.
func hasFileChangesTree(g model.TargetGraph, target model.TargetSpec, stateSet store.BuildStateSet) ([]model.TargetID, error) {
	result := []model.TargetID{}
	err := g.VisitTree(target, func(current model.TargetSpec) error {
		state := stateSet[current.ID()]
		if len(state.FilesChangedSet) > 0 {
			result = append(result, current.ID())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Given a target, return all the files in its tree of dependencies that have
// changed.
func filesChangedTree(g model.TargetGraph, target model.TargetSpec, stateSet store.BuildStateSet) ([]string, error) {
	result := []string{}
	err := g.VisitTree(target, func(current model.TargetSpec) error {
		state := stateSet[current.ID()]
		result = append(result, state.FilesChanged()...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sliceutils.DedupedAndSorted(result), nil
}
