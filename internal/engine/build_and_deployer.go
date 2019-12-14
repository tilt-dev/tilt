package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
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

func (bo BuildOrder) String() string {
	var output strings.Builder
	output.WriteString("BuildOrder{")

	for _, b := range bo {
		output.WriteString(fmt.Sprintf(" %T", b))
	}

	output.WriteString(" }")

	return output.String()
}

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
	var lastErr, lastUnexpectedErr error
	logger.Get(ctx).Debugf("Building with BuildOrder: %s", composite.builders.String())
	for i, builder := range composite.builders {
		logger.Get(ctx).Debugf("Trying to build and deploy with %T", builder)
		br, err := builder.BuildAndDeploy(ctx, st, specs, currentState)
		if err == nil {
			return br, err
		}

		if !buildcontrol.ShouldFallBackForErr(err) {
			return br, err
		}

		if redirectErr, ok := err.(buildcontrol.RedirectToNextBuilder); ok {
			s := fmt.Sprintf("falling back to next update method because: %v\n", err)
			logger.Get(ctx).Write(redirectErr.Level, []byte(s))
		} else {
			lastUnexpectedErr = err
			if i+1 < len(composite.builders) {
				logger.Get(ctx).Infof("got unexpected error during build/deploy: %v", err)
			}
		}
		lastErr = err
	}

	if lastUnexpectedErr != nil {
		// The most interesting error is the last UNEXPECTED error we got
		return store.BuildResultSet{}, lastUnexpectedErr
	}
	return store.BuildResultSet{}, lastErr
}

func DefaultBuildOrder(lubad *LiveUpdateBuildAndDeployer, ibad *ImageBuildAndDeployer, dcbad *DockerComposeBuildAndDeployer,
	ltbad *LocalTargetBuildAndDeployer, updMode buildcontrol.UpdateMode, env k8s.Env, runtime container.Runtime) BuildOrder {
	if updMode == buildcontrol.UpdateModeImage || updMode == buildcontrol.UpdateModeNaive {
		return BuildOrder{dcbad, ibad, ltbad}
	}

	if updMode == buildcontrol.UpdateModeSynclet || shouldUseSynclet(updMode, env, runtime) {
		ibad.SetInjectSynclet(true)
	}

	return BuildOrder{lubad, dcbad, ibad, ltbad}
}

func shouldUseSynclet(updMode buildcontrol.UpdateMode, env k8s.Env, runtime container.Runtime) bool {
	return updMode == buildcontrol.UpdateModeAuto && !env.UsesLocalDockerRegistry() && runtime == container.RuntimeDocker
}
