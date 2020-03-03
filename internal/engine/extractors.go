package engine

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
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

// If there are images that can be updated in-place in a container, return
// a state tree of what needs to be updated.
func extractImageTargetsForLiveUpdates(specs []model.TargetSpec, stateSet store.BuildStateSet) ([]liveUpdateStateTree, error) {
	g, err := model.NewTargetGraph(specs)
	if err != nil {
		return nil, errors.Wrap(err, "extractImageTargetsForLiveUpdates")
	}

	if !g.IsSingleSourceDAG() {
		return nil, fmt.Errorf("Cannot extract live updates on this build graph structure")
	}

	result := make([]liveUpdateStateTree, 0)

	deployedImages := g.DeployedImages()
	for _, iTarget := range deployedImages {
		state := stateSet[iTarget.ID()]
		if state.IsEmpty() {
			return nil, buildcontrol.SilentRedirectToNextBuilderf("In-place build does not support initial deploy")
		}

		if state.ImageBuildTriggered {
			return nil, buildcontrol.SilentRedirectToNextBuilderf("Force update (triggered manually, not automatically, with no dirty files)")
		}

		hasFileChangesIDs, err := hasFileChangesTree(g, iTarget, stateSet)
		if err != nil {
			return nil, errors.Wrap(err, "extractImageTargetsForLiveUpdates")
		}

		// If this image and none of its dependencies need a rebuild,
		// we can skip it.
		if len(hasFileChangesIDs) == 0 {
			continue
		}

		luInfo := iTarget.LiveUpdateInfo()
		if luInfo.Empty() {
			return nil, buildcontrol.SilentRedirectToNextBuilderf("LiveUpdate requires that LiveUpdate details be specified")
		}

		if state.RunningContainerError != nil {
			return nil, buildcontrol.RedirectToNextBuilderInfof("Error retrieving container info: %v", state.RunningContainerError)
		}

		// Now that we have live update information, we know this CAN be updated in
		// a container(s). Check to see if we have enough information about the
		// container(s) that would need to be updated.
		if len(state.RunningContainers) == 0 {
			return nil, buildcontrol.RedirectToNextBuilderInfof("Don't have info for running container of image %q "+
				"(often a result of the deployment not yet being ready)", container.FamiliarString(iTarget.Refs.ClusterRef()))
		}

		filesChanged, err := filesChangedTree(g, iTarget, stateSet)
		if err != nil {
			return nil, errors.Wrap(err, "extractImageTargetsForLiveUpdates")
		}

		result = append(result, liveUpdateStateTree{
			iTarget:           iTarget,
			filesChanged:      filesChanged,
			iTargetState:      state,
			hasFileChangesIDs: hasFileChangesIDs,
		})
	}

	return result, nil
}

// Returns true if the given image is deployed to one of the given k8s targets.
// Note that some images are injected into other images, so may never be deployed.
func isImageDeployedToK8s(iTarget model.ImageTarget, kTarget model.K8sTarget) bool {
	id := iTarget.ID()
	for _, depID := range kTarget.DependencyIDs() {
		if depID == id {
			return true
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
