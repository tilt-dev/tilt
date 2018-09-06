package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/k8s"
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

	// BaD needs to be able to get the container that a given build affected, so that
	// it can do incremental builds on that container if needed.
	// NOTE(maia): this isn't quite relevant to ImageBuildAndDeployer,
	// consider putting elsewhere or renaming (`Warm`?).
	GetContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error)
}
