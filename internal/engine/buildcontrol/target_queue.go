package buildcontrol

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Allows the caller to inject its own build strategy for dirty targets.
type BuildHandler func(
	target model.TargetSpec,
	depResults []store.BuildResult) (store.BuildResult, error)

type ReuseRefChecker func(ctx context.Context, iTarget model.ImageTarget, namedTagged reference.NamedTagged) (bool, error)

// A little data structure to help iterate through dirty targets in dependency order.
type TargetQueue struct {
	sortedTargets []model.TargetSpec

	// The state from the previous build.
	// Contains files-changed so that we can recycle old builds.
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

func NewImageTargetQueue(ctx context.Context, iTargets []model.ImageTarget, state store.BuildStateSet, canReuseRef ReuseRefChecker) (*TargetQueue, error) {
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
		} else if state[id].LastResult != nil {
			image := store.LocalImageRefFromBuildResult(state[id].LastResult)
			ok, err := canReuseRef(ctx, target.(model.ImageTarget), image)
			if err != nil {
				return nil, errors.Wrapf(err, "error looking up whether last image built for %s exists", image.String())
			}
			if !ok {
				logger.Get(ctx).Infof("Rebuilding %s because image not found in image store", container.FamiliarString(image))
				needsOwnBuild[id] = true
			}
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
	queue := &TargetQueue{
		sortedTargets: sortedTargets,
		state:         state,
		results:       results,
		needsOwnBuild: needsOwnBuild,
		depsNeedBuild: depsNeedBuild,
	}
	err = queue.backfillExistingResults()
	if err != nil {
		return nil, err
	}
	return queue, nil
}

// New results that were built with the current queue. Omits results
// that were re-used previous builds.
//
// Returns results that the BuildAndDeploy contract expects.
func (q *TargetQueue) NewResults() store.BuildResultSet {
	newResults := store.BuildResultSet{}
	for id, result := range q.results {
		if q.isBuilding(id) {
			newResults[id] = result
		}
	}
	return newResults
}

// Reused results that were not built with the current queue.
//
// Used for printing out which builds are cached from previous builds.
func (q *TargetQueue) ReusedResults() store.BuildResultSet {
	reusedResults := store.BuildResultSet{}
	for id, result := range q.results {
		if !q.isBuilding(id) {
			reusedResults[id] = result
		}
	}
	return reusedResults
}

// All results for targets in the current queue.
func (q *TargetQueue) AllResults() store.BuildResultSet {
	allResults := store.BuildResultSet{}
	for id, result := range q.results {
		allResults[id] = result
	}
	return allResults
}

func (q *TargetQueue) isBuilding(id model.TargetID) bool {
	return q.needsOwnBuild[id] || q.depsNeedBuild[id]
}

func (q *TargetQueue) CountBuilds() int {
	result := 0
	for _, target := range q.sortedTargets {
		if q.isBuilding(target.ID()) {
			result++
		}
	}
	return result
}

func (q *TargetQueue) backfillExistingResults() error {
	for _, target := range q.sortedTargets {
		id := target.ID()
		if !q.isBuilding(id) {
			// We can re-use results from the previous build.
			lastResult := q.state[id].LastResult
			image := store.LocalImageRefFromBuildResult(lastResult)
			if image == nil {
				return fmt.Errorf("Internal error: build marked clean but last result not found: %+v", q.state[id])
			}
			q.results[id] = lastResult
		}
	}
	return nil
}

func (q *TargetQueue) RunBuilds(handler BuildHandler) error {
	for _, target := range q.sortedTargets {
		id := target.ID()
		if q.isBuilding(id) {
			result, err := handler(target, q.dependencyResults(target))
			if err != nil {
				return err
			}
			q.results[id] = result
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
