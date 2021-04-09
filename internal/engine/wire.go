// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/wire"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
)

var DeployerBaseWireSet = wire.NewSet(
	buildcontrol.BaseWireSet,
	wire.Value(UpperReducer),

	// BuildOrder
	NewLiveUpdateBuildAndDeployer,
	DefaultBuildOrder,

	wire.Bind(new(buildcontrol.BuildAndDeployer), new(*CompositeBuildAndDeployer)),
	NewCompositeBuildAndDeployer,
)

var DeployerWireSetTest = wire.NewSet(
	DeployerBaseWireSet,
	wire.InterfaceValue(new(sdktrace.SpanProcessor), (sdktrace.SpanProcessor)(nil)),
)

var DeployerWireSet = wire.NewSet(
	DeployerBaseWireSet,
)

func provideBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	dir *dirs.TiltDevDir,
	env k8s.Env,
	updateMode buildcontrol.UpdateModeFlag,
	dcc dockercompose.DockerComposeClient,
	clock build.Clock,
	kp buildcontrol.KINDLoader,
	analytics *analytics.TiltAnalytics) (buildcontrol.BuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}
