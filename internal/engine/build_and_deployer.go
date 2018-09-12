package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet"
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

// CompositeBuildAndDeployer first attempts to build and deploy with the FirstLineBuildAndDeployer.
// If this fails, and the error returned isn't critical, we fall back to the FallbackBaD.
type CompositeBuildAndDeployer struct {
	firstLine      FirstLineBuildAndDeployer
	fallback       FallbackBuildAndDeployer
	shouldFallBack func(error) bool
}

func DefaultShouldFallBack() func(error) bool {
	return shouldImageBuild
}
func NewCompositeBuildAndDeployer(firstLine FirstLineBuildAndDeployer, fallback FallbackBuildAndDeployer,
	shouldFallBack func(error) bool) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{
		firstLine:      firstLine,
		fallback:       fallback,
		shouldFallBack: shouldFallBack,
	}
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, currentState BuildState) (BuildResult, error) {
	br, err := composite.firstLine.BuildAndDeploy(ctx, service, currentState)
	if err == nil {
		return br, err
	}
	if composite.shouldFallBack(err) {
		logger.Get(ctx).Verbosef("falling back to secondary build and deploy method after error: %v", err)
		return composite.fallback.BuildAndDeploy(ctx, service, currentState)
	}
	return BuildResult{}, err
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
	return composite.firstLine.GetContainerForBuild(ctx, build)
}

func NewFirstLineBuildAndDeployer(sCli synclet.SyncletClient, cu *build.ContainerUpdater, env k8s.Env, kCli k8s.Client) FirstLineBuildAndDeployer {
	if env == k8s.EnvGKE {
		return NewSyncletBuildAndDeployer(sCli, kCli)
	}
	return NewLocalContainerBuildAndDeployer(cu, env, kCli)
}
