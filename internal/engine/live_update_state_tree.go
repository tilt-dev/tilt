package engine

import (
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

// A helper data structure that represents a live-update image and
// the files changed in all of its dependencies.
type liveUpdateStateTree struct {
	iTarget           model.ImageTarget
	filesChanged      []string
	iTargetState      store.BuildState
	hasFileChangesIDs []model.TargetID
}

// Create a successful build result if the live update deploys successfully.
func (t liveUpdateStateTree) createResultSet() store.BuildResultSet {
	iTargetID := t.iTarget.ID()
	state := t.iTargetState
	res := state.LastResult

	res.LiveUpdatedContainerIDs = nil
	for _, c := range state.RunningContainers {
		res.LiveUpdatedContainerIDs = append(res.LiveUpdatedContainerIDs, c.ContainerID)
	}

	resultSet := store.BuildResultSet{}
	resultSet[iTargetID] = res

	// Invalidate all the image builds for images we depend on.
	// Otherwise, the image builder will think the existing image ID
	// is valid and won't try to rebuild it.
	for _, id := range t.hasFileChangesIDs {
		if id != iTargetID {
			resultSet[id] = store.BuildResult{}
		}
	}

	return resultSet
}

func createResultSet(trees []liveUpdateStateTree) store.BuildResultSet {
	resultSet := store.BuildResultSet{}
	for _, t := range trees {
		resultSet = store.MergeBuildResultsSet(resultSet, t.createResultSet())
	}
	return resultSet
}

func allChangedFiles(trees []liveUpdateStateTree) map[string]bool {
	res := make(map[string]bool)
	for _, t := range trees {
		for _, f := range t.filesChanged {
			res[f] = true
		}
	}
	return res
}
