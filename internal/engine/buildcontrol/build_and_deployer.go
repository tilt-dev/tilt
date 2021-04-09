package buildcontrol

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type BuildAndDeployer interface {
	// BuildAndDeploy builds and deployed the specified target specs.

	// Returns a BuildResultSet containing output (build result and associated
	// file changes) for each target built in this call. The BuildResultSet only
	// contains results for newly built targets--if a target was clean and didn't
	// need to be built, it doesn't appear in the result set.
	//
	// BuildResult can be used to construct a set of BuildStates, which contain
	// the last successful builds of each target and the files changed since that
	// build.
	BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error)
}
