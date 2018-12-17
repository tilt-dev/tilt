package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"

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
	BuildAndDeploy(ctx context.Context, manifest model.Manifest, currentState store.BuildState) (store.BuildResult, error)

	// PostProcessBuild gets any info about the build that we'll need for subsequent builds.
	// In general, we'll store this info ON the BuildAndDeployer that needs it.
	// Each implementation of PostProcessBuild is responsible for executing long-running steps async.
	PostProcessBuild(ctx context.Context, result, prevResult store.BuildResult)
}

type BuildOrder []BuildAndDeployer
type FallbackTester func(error) bool

// CompositeBuildAndDeployer tries to run each builder in order.  If a builder
// emits an error, it uses the FallbackTester to determine whether the error is
// critical enough to stop the whole pipeline, or to fallback to the next
// builder.
type CompositeBuildAndDeployer struct {
	builders BuildOrder
}

var _ BuildAndDeployer = &CompositeBuildAndDeployer{}

func NewCompositeBuildAndDeployer(builders BuildOrder) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{builders: builders}
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, currentState store.BuildState) (store.BuildResult, error) {
	var lastErr error
	for _, builder := range composite.builders {
		br, err := builder.BuildAndDeploy(ctx, manifest, currentState)
		if err == nil {
			// TODO(maia): this should be reactive (i.e. happen as a response to `BuildCompleteAction`)
			composite.PostProcessBuild(ctx, br, currentState.LastResult)
			return br, err
		}

		if !shouldFallBackForErr(err) {
			return store.BuildResult{}, err
		}

		if _, ok := err.(RedirectToNextBuilder); ok {
			logger.Get(ctx).Debugf("(expected error) falling back to next build and deploy method "+
				"after error: %v", err)
		} else {
			logger.Get(ctx).Verbosef("falling back to next build and deploy method "+
				"after unexpected error: %v", err)
		}

		lastErr = err
	}
	return store.BuildResult{}, lastErr
}

func (composite *CompositeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, prevResult store.BuildResult) {
	for _, builder := range composite.builders {
		builder.PostProcessBuild(ctx, result, prevResult)
	}
}

func DefaultBuildOrder(sbad *SyncletBuildAndDeployer, cbad *LocalContainerBuildAndDeployer, ibad *ImageBuildAndDeployer, dcbad *DockerComposeBuildAndDeployer, env k8s.Env, mode UpdateMode) BuildOrder {

	if mode == UpdateModeImage || mode == UpdateModeNaive {
		return BuildOrder{dcbad, ibad}
	}

	if mode == UpdateModeContainer {
		return BuildOrder{cbad, dcbad, ibad}
	}

	if mode == UpdateModeSynclet {
		ibad.SetInjectSynclet(true)
		return BuildOrder{sbad, dcbad, ibad}
	}

	if env.IsLocalCluster() {
		return BuildOrder{cbad, dcbad, ibad}
	}

	ibad.SetInjectSynclet(true)
	return BuildOrder{sbad, dcbad, ibad}
}
