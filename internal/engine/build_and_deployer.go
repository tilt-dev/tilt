package engine

import (
	"context"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type BuildAndDeployer interface {
	// BuildAndDeploy builds and deployed the specified manifest.
	//
	// Returns a BuildResult that expresses the output of the build.
	//
	// BuildResult can be used to construct a BuildState, which contains
	// the last successful build and the files changed since that build.
	BuildAndDeploy(ctx context.Context, manifest model.Manifest, currentState BuildState) (BuildResult, error)

	// PostProcessBuild gets any info about the build that we'll need for subsequent builds.
	// In general, we'll store this info ON the BuildAndDeployer that needs it.
	// Each implementation of PostProcessBuild is responsible for executing long-running steps async.
	PostProcessBuild(ctx context.Context, result, prevResult BuildResult)
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

var _ BuildAndDeployer = &CompositeBuildAndDeployer{}

func DefaultShouldFallBack() FallbackTester {
	return FallbackTester(shouldImageBuild)
}

func NewCompositeBuildAndDeployer(builders BuildOrder, shouldFallBack FallbackTester) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{
		builders:       builders,
		shouldFallBack: shouldFallBack,
	}
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, currentState BuildState) (BuildResult, error) {
	var lastErr error
	for _, builder := range composite.builders {
		br, err := builder.BuildAndDeploy(ctx, manifest, currentState)
		if err == nil {
			// TODO(maia): maybe this only needs to be called after certain builds?
			// I.e. should be called after image build but not after a successful container build?
			composite.PostProcessBuild(ctx, br, currentState.LastResult)
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

// A permanent error indicates that the whole build pipeline needs to stop.
// It will never recover, even on subsequent rebuilds.
func isPermanentError(err error) bool {
	if _, ok := err.(*model.ValidateErr); ok {
		return true
	}

	cause := errors.Cause(err)
	if cause == context.Canceled {
		return true
	}
	return false
}

// Given the error from our initial BuildAndDeploy attempt, shouldImageBuild determines
// whether we should fall back to an ImageBuild.
func shouldImageBuild(err error) bool {
	if _, ok := err.(*build.PathMappingErr); ok {
		return false
	}
	if isPermanentError(err) {
		return false
	}

	if build.IsUserBuildFailure(err) {
		return false
	}
	return true
}

func (composite *CompositeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, prevResult BuildResult) {
	for _, builder := range composite.builders {
		builder.PostProcessBuild(ctx, result, prevResult)
	}
}

func DefaultBuildOrder(sbad *SyncletBuildAndDeployer, cbad *LocalContainerBuildAndDeployer, ibad *ImageBuildAndDeployer, env k8s.Env, mode UpdateMode) BuildOrder {
	if mode == UpdateModeImage || mode == UpdateModeNaive {
		return BuildOrder{ibad}
	}

	if mode == UpdateModeContainer {
		return BuildOrder{cbad, ibad}
	}

	if mode == UpdateModeSynclet {
		ibad.SetInjectSynclet(true)
		return BuildOrder{sbad, ibad}
	}

	if env.IsLocalCluster() {
		return BuildOrder{cbad, ibad}
	}

	ibad.SetInjectSynclet(true)
	return BuildOrder{sbad, ibad}
}
