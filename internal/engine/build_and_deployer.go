package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
)

type BuildAndDeployer interface {
	// Builds and deployed the specified service.
	//
	// Returns a BuildResult that expresses the output of the build.
	//
	// BuildResult can be used to construct a BuildState, which contains
	// the last successful build and the files changed since that build.
	BuildAndDeploy(ctx context.Context, service model.Service, currentState BuildState) (BuildResult, error)
}
