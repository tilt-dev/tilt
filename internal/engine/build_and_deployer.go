package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type BuildAndDeployer interface {
	// BuildAndDeploy builds and deployed the specified target specs.
	//
	// Returns a BuildResult that expresses the outputs(s) of the build.
	//
	// BuildResult can be used to construct a set of BuildStates, which contain
	// the last successful builds of each target and the files changed since that build.
	BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error)
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

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error) {
	var lastErr error
	for _, builder := range composite.builders {
		br, err := builder.BuildAndDeploy(ctx, st, specs, currentState)
		if err == nil {
			return br, err
		}

		if !shouldFallBackForErr(err) {
			return store.BuildResultSet{}, err
		}

		if _, ok := err.(RedirectToNextBuilder); ok {
			logger.Get(ctx).Debugf("(expected error) falling back to next build and deploy method "+
				"after error: %v", err)
		} else {
			logger.Get(ctx).Infof("falling back to next build and deploy method "+
				"after unexpected error: %v", err)
		}

		lastErr = err
	}
	return store.BuildResultSet{}, lastErr
}

func DefaultBuildOrder(sbad *SyncletBuildAndDeployer, cbad *LocalContainerBuildAndDeployer, ibad *ImageBuildAndDeployer, dcbad *DockerComposeBuildAndDeployer, env k8s.Env, updMode UpdateMode, runtime container.Runtime) BuildOrder {

	if updMode == UpdateModeImage || updMode == UpdateModeNaive {
		return BuildOrder{dcbad, ibad}
	}

	if updMode == UpdateModeContainer {
		return BuildOrder{cbad, dcbad, ibad}
	}

	if updMode == UpdateModeSynclet {
		if runtime == container.RuntimeDocker {
			ibad.SetInjectSynclet(true)
		}
		return BuildOrder{sbad, dcbad, ibad}
	}

	if env.IsLocalCluster() && runtime == container.RuntimeDocker {
		return BuildOrder{cbad, dcbad, ibad}
	}

	if runtime == container.RuntimeDocker {
		ibad.SetInjectSynclet(true)
	}
	return BuildOrder{sbad, dcbad, ibad}
}
