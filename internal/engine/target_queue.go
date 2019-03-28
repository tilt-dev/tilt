package engine

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

// Allows the caller to inject its own build strategy for dirty targets.
type BuildHandler func(
	target model.TargetSpec,
	state store.BuildState,
	depResults []store.BuildResult) (store.BuildResult, error)

// A little data structure to help iterate through dirty targets in dependency order.
type TargetQueue struct {
	sortedTargets []model.TargetSpec

	// The state from the previous build.
	// Contains files-changed so we can do incremental builds.
	state store.BuildStateSet

	// The results of this build.
	results store.BuildResultSet

	// Whether the target itself needs a rebuilt, either because it has dirty files
	// or has never been built before.
	//
	// A target with dirty files might be able to use the files changed
	// since the previous result to build the next result.
	needsOwnBuild map[model.TargetID]bool

	// Whether the target depends transitively on something that needs rebuilding.
	// A target that depends on a dirty target should never use its previous
	// result to build the next result.
	depsNeedBuild map[model.TargetID]bool
}

func NewImageTargetQueue(iTargets []model.ImageTarget, state store.BuildStateSet) (*TargetQueue, error) {
	targets := make([]model.TargetSpec, 0, len(iTargets))
	for _, iTarget := range iTargets {
		targets = append(targets, iTarget)
	}

	sortedTargets, err := model.TopologicalSort(targets)
	if err != nil {
		return nil, err
	}

	needsOwnBuild := make(map[model.TargetID]bool)
	for _, target := range sortedTargets {
		id := target.ID()
		if state[id].NeedsImageBuild() {
			needsOwnBuild[id] = true
		}
	}

	depsNeedBuild := make(map[model.TargetID]bool)
	for _, target := range sortedTargets {
		for _, depID := range target.DependencyIDs() {
			if needsOwnBuild[depID] || depsNeedBuild[depID] {
				depsNeedBuild[target.ID()] = true
				break
			}
		}
	}

	results := make(store.BuildResultSet, len(targets))
	return &TargetQueue{
		sortedTargets: sortedTargets,
		state:         state,
		results:       results,
		needsOwnBuild: needsOwnBuild,
		depsNeedBuild: depsNeedBuild,
	}, nil
}

func (q *TargetQueue) CountDirty() int {
	result := 0
	for _, target := range q.sortedTargets {
		if q.needsOwnBuild[target.ID()] || q.depsNeedBuild[target.ID()] {
			result++
		}
	}
	return result
}

func (q *TargetQueue) RunBuilds(handler BuildHandler) error {
	for _, target := range q.sortedTargets {
		id := target.ID()
		if q.depsNeedBuild[id] {
			// If the dependencies are dirty, we can't use any state from the previous build.
			result, err := handler(target, store.BuildState{}, q.dependencyResults(target))
			if err != nil {
				return err
			}
			q.results[id] = result
		} else if q.needsOwnBuild[id] {
			// If only files are dirty, we can try to do an incremental build.
			result, err := handler(target, q.state[id], q.dependencyResults(target))
			if err != nil {
				return err
			}
			q.results[id] = result
		} else {
			// Otherwise, we can re-use results from the previous build.
			// If needsOwnBuild is false, then LastResult must exist if it's empty.
			lastResult := q.state[id].LastResult
			if lastResult.Image == nil {
				return fmt.Errorf("Internal error: build marked clean but last result not found: %+v", q.state[id])
			}
			q.results[id] = lastResult
		}
	}
	return nil
}

func (q *TargetQueue) dependencyResults(target model.TargetSpec) []store.BuildResult {
	depIDs := target.DependencyIDs()
	results := make([]store.BuildResult, 0, len(depIDs))
	for _, depID := range depIDs {
		results = append(results, q.results[depID])
	}
	return results
}
