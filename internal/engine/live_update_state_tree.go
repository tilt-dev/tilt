package engine

import (
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
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
	res := state.LastSuccessfulResult

	liveUpdatedContainerIDs := []container.ID{}
	for _, c := range state.RunningContainers {
		liveUpdatedContainerIDs = append(liveUpdatedContainerIDs, c.ContainerID)
	}

	resultSet := store.BuildResultSet{}
	resultSet[iTargetID] = store.NewLiveUpdateBuildResult(res.TargetID(), liveUpdatedContainerIDs)

	// Invalidate all the image builds for images we depend on.
	// Otherwise, the image builder will think the existing image ID
	// is valid and won't try to rebuild it.
	for _, id := range t.hasFileChangesIDs {
		if id != iTargetID {
			resultSet[id] = nil
		}
	}

	return resultSet
}

func createResultSet(trees []liveUpdateStateTree, luInfos []liveUpdInfo) store.BuildResultSet {
	liveUpdatedTargetIDs := make(map[model.TargetID]bool)
	for _, info := range luInfos {
		liveUpdatedTargetIDs[info.iTarget.ID()] = true
	}

	resultSet := store.BuildResultSet{}
	for _, t := range trees {
		if !liveUpdatedTargetIDs[t.iTarget.ID()] {
			// We didn't actually do a LiveUpdate for this tree
			continue
		}
		resultSet = store.MergeBuildResultsSet(resultSet, t.createResultSet())
	}
	return resultSet
}
