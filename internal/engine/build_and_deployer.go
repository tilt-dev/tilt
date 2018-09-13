package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type BuildAndDeployer interface {
	// Builds and deployed the specified service.
	//
	// Returns a BuildResult that expresses the output of the build.
	//
	// BuildResult can be used to construct a BuildState, which contains
	// the last successful build and the files changed since that build.
	BuildAndDeploy(ctx context.Context, service model.Manifest, currentState BuildState) (BuildResult, error)

	// BaD needs to be able to get the container that a given build affected, so that
	// it can do incremental builds on that container if needed.
	// NOTE(maia): this isn't quite relevant to ImageBuildAndDeployer,
	// consider putting elsewhere or renaming (`Warm`?).
	GetContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error)
}

type BuildOrder []BuildAndDeployer
type FallbackTester func(error) bool

// CompositeBuildAndDeployer tries to run each builder in order.  If a builder
// emits an error, it uses the FallbackTester to determine whether the error is
// critical enough to stop the whole pipeline, or to fallback to the next
// builder.
type CompositeBuildAndDeployer struct {
	builders       BuildOrder
	shouldFallBack FallbackTester
}

func DefaultShouldFallBack() FallbackTester {
	return FallbackTester(shouldImageBuild)
}

func NewCompositeBuildAndDeployer(builders BuildOrder, shouldFallBack FallbackTester) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{
		builders:       builders,
		shouldFallBack: shouldFallBack,
	}
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Manifest, currentState BuildState) (BuildResult, error) {
	var lastErr error
	for _, builder := range composite.builders {
		br, err := builder.BuildAndDeploy(ctx, service, currentState)
		if err == nil {
			return br, err
		}

		if !composite.shouldFallBack(err) {
			return BuildResult{}, err
		}
		logger.Get(ctx).Verbosef("falling back to next build and deploy method after error: %v", err)
		lastErr = err
	}
	return BuildResult{}, lastErr
}

// Given the error from our initial BuildAndDeploy attempt, shouldImageBuild determines
// whether we should fall back to an ImageBuild.
func shouldImageBuild(err error) bool {
	if _, ok := err.(*build.PathMappingErr); ok {
		return false
	}
	if _, ok := err.(*model.ValidateErr); ok {
		return false
	}
	return true
}

func (composite *CompositeBuildAndDeployer) GetContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	// NOTE(maia): this will be relocated soon... for now, call out to the embedded BaD that has this implemented
	var lastErr error
	for _, builder := range composite.builders {
		cID, err := builder.GetContainerForBuild(ctx, build)
		if err == nil {
			return cID, err
		}

		if !composite.shouldFallBack(err) {
			return "", err
		}
		logger.Get(ctx).Verbosef("falling back to next build and deploy method after error: %v", err)
		lastErr = err
	}
	return "", lastErr
}

func DefaultBuildOrder(sbad *SyncletBuildAndDeployer, cbad *LocalContainerBuildAndDeployer, ibad *ImageBuildAndDeployer, env k8s.Env) BuildOrder {
	switch env {
	case k8s.EnvMinikube, k8s.EnvDockerDesktop:
		return BuildOrder{cbad, ibad}
	default:
		return BuildOrder{sbad, ibad}
	}
}
