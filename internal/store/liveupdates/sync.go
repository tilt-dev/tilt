package liveupdates

import (
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// LiveUpdatePlan is the result of evaluating a list of changed files against the rules in the spec.
//
// TODO(milas): we should probably canonicalize all the local paths here using .spec.basePath
type LiveUpdatePlan struct {
	// SyncPaths are changed (local) paths and their corresponding container paths to be synced.
	SyncPaths []build.PathMapping
	// NoMatchPaths are changed (local) paths that do not match any Live Update rules.
	NoMatchPaths []string
	// FallBackPaths are changed (local) paths that matched a fallback path rule.
	FallBackPaths []string
}

// NewLiveUpdatePlan evaluates a set of changed files against a LiveUpdateSpec.
func NewLiveUpdatePlan(luSpec v1alpha1.LiveUpdateSpec, filesChanged []string) (LiveUpdatePlan, error) {
	var plan LiveUpdatePlan

	var err error
	plan.SyncPaths, plan.NoMatchPaths, err = build.FilesToPathMappings(
		filesChanged,
		liveupdate.SyncSteps(luSpec),
	)
	if err != nil {
		return LiveUpdatePlan{}, err
	}

	plan.FallBackPaths, err = liveupdate.FallBackOnFiles(luSpec).Intersection(filesChanged)
	if err != nil {
		return LiveUpdatePlan{}, err
	}

	return plan, nil
}
