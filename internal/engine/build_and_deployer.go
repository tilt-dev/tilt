package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/build"
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

type FirstLineBuildAndDeployer BuildAndDeployer
type FallbackBuildAndDeployer BuildAndDeployer

type CompositeBuildAndDeployer struct {
	containerBaD BuildAndDeployer
	imageBaD     BuildAndDeployer
}

func NewCompositeBuildAndDeployer(firstLine FirstLineBuildAndDeployer, fallback FallbackBuildAndDeployer) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{
		containerBaD: firstLine,
		imageBaD:     fallback,
	}
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, currentState BuildState) (BuildResult, error) {
	br, err := composite.containerBaD.BuildAndDeploy(ctx, service, currentState)
	if err == nil {
		return br, err
	}
	if shouldSkipImageBuild(err) {
		return BuildResult{}, err
	}
	return composite.imageBaD.BuildAndDeploy(ctx, service, currentState)
}

// shouldSkipImageBuild determines whether the given error (from the containerBuildAndDeployer)
// means that we should skip the image build.
func shouldSkipImageBuild(err error) bool {
	if _, ok := err.(*build.PathMappingErr); ok {
		return true
	}
	if _, ok := err.(*model.ValidateErr); ok {
		return true
	}
	return false
}

func (composite *CompositeBuildAndDeployer) GetContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	// NOTE(maia): this will be relocated soon... for now, call out to the embedded BaD that has this implemented
	return composite.containerBaD.GetContainerForBuild(ctx, build)
}
