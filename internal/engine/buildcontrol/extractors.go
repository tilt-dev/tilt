package buildcontrol

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
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

// If there are images that can be updated in-place in a container, return
// a state tree of what needs to be updated.
func extractImageTargetsForLiveUpdates(specs []model.TargetSpec, stateSet store.BuildStateSet) ([]liveUpdateStateTree, error) {
	// HACK(milas): LU-only targets are NOT dependencies of the deploy target
	// 	(there is no build); this breaks the Live Update DAG logic which wants
	// 	the deploy target (K8sTarget/DockerComposeTarget) as the root node with
	// 	ImageTargets as dependencies, which can then themselves have more
	// 	ImageTargets as dependencies for base images. As a workaround, these
	// 	are _removed_ from the spec list for the graph and then always included
	//  in the collection of deployed images considered for Live Update ops.
	// 	Moving forward, this will be a non-issue, as the API reconcilers are
	// 	designed to work independently, but this keeps compatibility for the
	// 	v1 Live Update (currently always used for Docker Compose and only if
	// 	a feature flag is explicitly disabled for K8s) and prevents a scary
	// 	error in the v2 case.
	var luOnlyTargets []model.ImageTarget
	specsForGraph := make([]model.TargetSpec, 0, len(specs))
	for _, spec := range specs {
		if iTarget, ok := spec.(model.ImageTarget); ok && iTarget.IsLiveUpdateOnly {
			luOnlyTargets = append(luOnlyTargets, iTarget)
		} else {
			// * any image targets that have a build (i.e. not LU-only)
			// * all other targets (e.g. K8sTarget, DockerComposeTarget)
			specsForGraph = append(specsForGraph, spec)
		}
	}

	g, err := model.NewTargetGraph(specsForGraph)
	if err != nil {
		return nil, errors.Wrap(err, "extractImageTargetsForLiveUpdates")
	}

	if !g.IsSingleSourceDAG() {
		return nil, fmt.Errorf("Cannot extract live updates on this build graph structure")
	}

	result := make([]liveUpdateStateTree, 0)

	deployedImages := g.DeployedImages()
	deployedImages = append(deployedImages, luOnlyTargets...)

	for _, iTarget := range deployedImages {
		state := stateSet[iTarget.ID()]
		// If this is a normal image build, it must have info about the deployed image.
		// Otherwise, we can update pods if we can find their containers.
		if !iTarget.IsLiveUpdateOnly && state.IsEmpty() {
			return nil, SilentRedirectToNextBuilderf("In-place build does not support initial deploy")
		}

		if state.FullBuildTriggered {
			return nil, SilentRedirectToNextBuilderf("Force update (triggered manually, not automatically, with no dirty files)")
		}

		if len(state.DepsChangedSet) > 0 {
			return nil, SilentRedirectToNextBuilderf("Pending dependencies")
		}

		hasFileChangesIDs, err := HasFileChangesTree(g, iTarget, stateSet)
		if err != nil {
			return nil, errors.Wrap(err, "extractImageTargetsForLiveUpdates")
		}

		// If this image and none of its dependencies need a rebuild,
		// we can skip it.
		if len(hasFileChangesIDs) == 0 {
			continue
		}

		luSpec := iTarget.LiveUpdateSpec
		if liveupdate.IsEmptySpec(luSpec) {
			return nil, SilentRedirectToNextBuilderf("LiveUpdate requires that LiveUpdate details be specified")
		}

		// TODO(nick): In LiveUpdate v2, the reconciler will put a hold on all image builds
		// while it's handling file changes. We'll only get to this codepath if the reconciler
		// fails.
		//
		// If the reconciler fails, it should log some message about why it failed.
		// But currently, the BuildAndDeployer itself doesn't surface any info about why
		// live update didn't apply. A fix would be to add it to the BuildState and log it here.
		// But it might also be worth waiting until the Image reconciler to do this.
		if iTarget.LiveUpdateReconciler {
			return nil, SilentRedirectToNextBuilderf("Live update handled by API controller")
		}

		containers, err := liveupdates.RunningContainers(
			state.KubernetesSelector,
			state.KubernetesResource,
			state.DockerResource)

		if err != nil {
			return nil, RedirectToNextBuilderInfof("Error retrieving container info: %v", err)
		}

		// Now that we have live update information, we know this CAN be updated in
		// a container(s). Check to see if we have enough information about the
		// container(s) that would need to be updated.
		if len(containers) == 0 {
			return nil, RedirectToNextBuilderInfof("Don't have info for running container of image %q "+
				"(often a result of the deployment not yet being ready)", container.FamiliarString(iTarget.Refs.ClusterRef()))
		}

		filesChanged, err := filesChangedTree(g, iTarget, stateSet)
		if err != nil {
			return nil, errors.Wrap(err, "extractImageTargetsForLiveUpdates")
		}

		result = append(result, liveUpdateStateTree{
			iTarget:           iTarget,
			filesChanged:      filesChanged,
			containers:        containers,
			hasFileChangesIDs: hasFileChangesIDs,
			isDC:              state.DockerResource != nil,
		})
	}

	return result, nil
}

// Returns true if the given image is deployed to one of the given k8s targets.
// Note that some images are injected into other images, so may never be deployed.
func IsImageDeployedToK8s(iTarget model.ImageTarget, kTarget model.K8sTarget) bool {
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
func HasFileChangesTree(g model.TargetGraph, target model.TargetSpec, stateSet store.BuildStateSet) ([]model.TargetID, error) {
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
