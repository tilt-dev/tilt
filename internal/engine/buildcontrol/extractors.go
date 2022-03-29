package buildcontrol

import (
	"github.com/tilt-dev/tilt/internal/store"
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
