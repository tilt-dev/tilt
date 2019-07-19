package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/windmilleng/tilt/internal/engine/errors"
	"github.com/windmilleng/tilt/internal/mode"
	"github.com/windmilleng/tilt/internal/store"

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

func (bo BuildOrder) String() string {
	var output strings.Builder
	output.WriteString("BuildOrder{")

	for _, b := range bo {
		output.WriteString(fmt.Sprintf(" %T", b))
	}

	output.WriteString(" }")

	return output.String()
}

// Differentiate between the BuildOrder we'll use for k8s targets and for dc targets
type K8sOrder BuildOrder
type DCOrder BuildOrder

type FallbackTester func(error) bool

// CompositeBuildAndDeployer tries to run each builder in order.  If a builder
// emits an error, it uses the FallbackTester to determine whether the error is
// critical enough to stop the whole pipeline, or to fallback to the next
// builder.
type CompositeBuildAndDeployer struct {
	k8sBuilders BuildOrder
	dcBuilders  BuildOrder // if any DC targets present, use this order
}

var _ BuildAndDeployer = &CompositeBuildAndDeployer{}

func NewCompositeBuildAndDeployer(k8sOrder K8sOrder, dcOrder DCOrder) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{k8sBuilders: BuildOrder(k8sOrder), dcBuilders: BuildOrder(dcOrder)}
}

// BuildOrderForSpecs returns the sequence of builders we should use for these targets.
// If any DC targets are present, use the dcBuilders; otherwise, the k8sBuilders.
func (composite *CompositeBuildAndDeployer) buildOrderForSpecs(specs []model.TargetSpec) BuildOrder {
	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	if isDC {
		return composite.dcBuilders
	}
	return composite.k8sBuilders
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error) {
	builders := composite.buildOrderForSpecs(specs)

	var lastErr, lastUnexpectedErr error
	logger.Get(ctx).Debugf("Building with BuildOrder: %s", builders.String())
	for i, builder := range builders {
		logger.Get(ctx).Debugf("Trying to build and deploy with %T", builder)
		br, err := builder.BuildAndDeploy(ctx, st, specs, currentState)
		if err == nil {
			return br, err
		}

		if !errors.ShouldFallBackForErr(err) {
			return store.BuildResultSet{}, err
		}

		if redirectErr, ok := err.(errors.RedirectToNextBuilder); ok {
			s := fmt.Sprintf("falling back to next update method because: %v\n", err)
			logger.Get(ctx).Write(redirectErr.Level, s)
		} else {
			lastUnexpectedErr = err
			if i+1 < len(builders) {
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

func DefaultBuildOrderForK8s(liveUpdBAD k8sLiveUpdBAD, ibad *ImageBuildAndDeployer, dcbad *DockerComposeBuildAndDeployer, updMode mode.UpdateMode) K8sOrder {
	if updMode == mode.UpdateModeImage || updMode == mode.UpdateModeNaive {
		return K8sOrder(BuildOrder{ibad})
	}

	k8sLiveUpdater := LiveUpdateBuildAndDeployer(*liveUpdBAD)
	if k8sLiveUpdater.IsSyncletUpdater() {
		ibad.NewWithInjectSynclet(true)
	}

	return K8sOrder(BuildOrder{&k8sLiveUpdater, dcbad, ibad})
}

func DefaultBuildOrderForDC(liveUpdBAD dcLiveUpdBAD, ibad *ImageBuildAndDeployer, dcbad *DockerComposeBuildAndDeployer, updMode mode.UpdateMode) DCOrder {
	if updMode == mode.UpdateModeImage || updMode == mode.UpdateModeNaive {
		return DCOrder(BuildOrder{dcbad})
	}

	dcLiveUpdater := LiveUpdateBuildAndDeployer(*liveUpdBAD)
	return DCOrder(BuildOrder{&dcLiveUpdater, dcbad, ibad})
}
