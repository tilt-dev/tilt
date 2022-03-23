package engine

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type BuildOrder []buildcontrol.BuildAndDeployer

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
	tracer   trace.Tracer
}

var _ buildcontrol.BuildAndDeployer = &CompositeBuildAndDeployer{}

func NewCompositeBuildAndDeployer(builders BuildOrder, tracer trace.Tracer) *CompositeBuildAndDeployer {
	return &CompositeBuildAndDeployer{builders: builders, tracer: tracer}
}

func (composite *CompositeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error) {
	ctx, span := composite.tracer.Start(ctx, "update")
	defer span.End()
	var lastErr, lastUnexpectedErr error

	specNames := []string{}

	for _, s := range specs {
		specNames = append(specNames, s.ID().String())
	}
	span.SetAttributes(attribute.KeyValue{Key: attribute.Key("targetNames"), Value: attribute.StringValue(strings.Join(specNames, ","))})

	logger.Get(ctx).Debugf("Building with BuildOrder: %s", composite.builders.String())
	for i, builder := range composite.builders {
		buildType := fmt.Sprintf("%T", builder)
		logger.Get(ctx).Debugf("Trying to build and deploy with %s", buildType)

		br, err := builder.BuildAndDeploy(ctx, st, specs, currentState)
		if err == nil {
			buildTypes := br.BuildTypes()
			for _, bt := range buildTypes {
				span.SetAttributes(attribute.KeyValue{Key: attribute.Key(fmt.Sprintf("buildType.%s", bt)), Value: attribute.BoolValue(true)})
			}
			return br, nil
		}

		if !buildcontrol.ShouldFallBackForErr(err) {
			return br, err
		}

		_, isLiveUpdate := builder.(*buildcontrol.LiveUpdateBuildAndDeployer)
		l := logger.Get(ctx).WithFields(logger.Fields{logger.FieldNameBuildEvent: "fallback"})

		if redirectErr, ok := err.(buildcontrol.RedirectToNextBuilder); ok {
			s := fmt.Sprintf("Falling back to next update method…\nREASON: %v\n", err)
			if isLiveUpdate && redirectErr.UserFacing() {
				s = fmt.Sprintf("Will not perform Live Update because:\n\t%v\n"+
					"Falling back to a full image build + deploy\n", err)
			}
			l.Write(redirectErr.Level, []byte(s))
		} else {
			lastUnexpectedErr = err
			if isLiveUpdate {
				// Indent the error message.
				errMsg := strings.ReplaceAll(strings.TrimSpace(fmt.Sprintf("%v", err)), "\n", "\n\t")
				l.Warnf("Live Update failed with unexpected error:\n\t%s\n"+
					"Falling back to a full image build + deploy", errMsg)
			} else if i+1 < len(composite.builders) {
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

func DefaultBuildOrder(lubad *buildcontrol.LiveUpdateBuildAndDeployer, ibad *buildcontrol.ImageBuildAndDeployer, dcbad *buildcontrol.DockerComposeBuildAndDeployer,
	ltbad *buildcontrol.LocalTargetBuildAndDeployer, updMode liveupdates.UpdateMode, env clusterid.Product, runtime container.Runtime) BuildOrder {
	if updMode == liveupdates.UpdateModeImage {
		return BuildOrder{dcbad, ibad, ltbad}
	}

	return BuildOrder{lubad, dcbad, ibad, ltbad}
}
