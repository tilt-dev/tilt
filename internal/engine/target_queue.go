package engine

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

// A little data structure to help iterate through targets in dependency order.
type TargetQueue struct {
	targets []model.TargetSpec
	results store.BuildResultSet
}

func NewImageTargetQueue(iTargets []model.ImageTarget) *TargetQueue {
	targets := make([]model.TargetSpec, 0, len(iTargets))
	for _, iTarget := range iTargets {
		targets = append(targets, iTarget)
	}

	return &TargetQueue{
		targets: targets,
		results: make(store.BuildResultSet, len(targets)),
	}
}

// Checks if we have results for all the dependencies of this target.
func (q *TargetQueue) isReady(target model.TargetSpec) bool {
	for _, depID := range target.DependencyIDs() {
		_, ok := q.results[depID]
		if !ok {
			return false
		}
	}
	return true
}

func (q *TargetQueue) SetResult(id model.TargetID, result store.BuildResult) {
	q.results[id] = result
}

func (q *TargetQueue) DependencyResults(target model.TargetSpec) []store.BuildResult {
	depIDs := target.DependencyIDs()
	results := make([]store.BuildResult, 0, len(depIDs))
	for _, depID := range depIDs {
		results = append(results, q.results[depID])
	}
	return results
}

func (q *TargetQueue) Next() (model.TargetSpec, bool, error) {
	for i, target := range q.targets {
		if q.isReady(target) {
			// Remove this target from the queue
			q.targets = append(q.targets[:i], q.targets[i+1:]...)
			return target, true, nil
		}
	}

	if len(q.targets) == 0 {
		return nil, false, nil
	}

	return nil, false, fmt.Errorf("Internal error: TargetQueue has unsatisfiable targets")
}
