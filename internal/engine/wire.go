// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/wire"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/synclet"
	"github.com/tilt-dev/tilt/internal/synclet/sidecar"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tracer"
)

var DeployerBaseWireSet = wire.NewSet(
	// dockerImageBuilder ( = ImageBuilder)
	wire.Value(dockerfile.Labels{}),
	wire.Value(UpperReducer),

	sidecar.WireSet,
	k8s.ProvideMinikubeClient,
	build.DefaultDockerBuilder,
	build.NewDockerImageBuilder,
	build.NewExecCustomBuilder,
	wire.Bind(new(build.CustomBuilder), new(*build.ExecCustomBuilder)),

	// BuildOrder
	NewLocalTargetBuildAndDeployer,
	NewImageBuildAndDeployer,
	containerupdate.NewDockerContainerUpdater,
	containerupdate.NewSyncletUpdater,
	containerupdate.NewExecUpdater,
	NewLiveUpdateBuildAndDeployer,
	NewDockerComposeBuildAndDeployer,
	NewImageBuilder,
	DefaultBuildOrder,

	tracer.InitOpenTelemetry,

	wire.Bind(new(BuildAndDeployer), new(*CompositeBuildAndDeployer)),
	NewCompositeBuildAndDeployer,
	buildcontrol.ProvideUpdateMode,
)

var DeployerWireSetTest = wire.NewSet(
	DeployerBaseWireSet,
	containerupdate.NewSyncletManagerForTests,
	wire.InterfaceValue(new(sdktrace.SpanProcessor), (sdktrace.SpanProcessor)(nil)),

	// A fake synclet wrapped in a GRPC interface
	synclet.FakeGRPCWrapper,
)

var DeployerWireSet = wire.NewSet(
	DeployerBaseWireSet,
	containerupdate.NewSyncletManager,
)

func provideBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	dir *dirs.WindmillDir,
	env k8s.Env,
	updateMode buildcontrol.UpdateModeFlag,
	sCli *synclet.TestSyncletClient,
	dcc dockercompose.DockerComposeClient,
	clock build.Clock,
	kp KINDLoader,
	analytics *analytics.TiltAnalytics) (BuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}

func provideImageBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	env k8s.Env,
	dir *dirs.WindmillDir,
	clock build.Clock,
	kp KINDLoader,
	analytics *analytics.TiltAnalytics) (*ImageBuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		wire.Value(buildcontrol.UpdateModeFlag(buildcontrol.UpdateModeAuto)),
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}

func provideKubectlLogLevelInfo() k8s.KubectlLogLevel {
	return k8s.KubectlLogLevel(0)
}

func provideDockerComposeBuildAndDeployer(
	ctx context.Context,
	dcCli dockercompose.DockerComposeClient,
	dCli docker.Client,
	dir *dirs.WindmillDir) (*DockerComposeBuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		wire.Value(buildcontrol.UpdateModeFlag(buildcontrol.UpdateModeAuto)),
		build.ProvideClock,
		provideKubectlLogLevelInfo,

		// EnvNone ensures that we get an exploding k8s client.
		wire.Value(k8s.Env(k8s.EnvNone)),
		k8s.ProvideClientConfig,
		k8s.ProvideConfigNamespace,
		k8s.ProvideKubeContext,
		k8s.ProvideKubectlRunner,
		k8s.ProvideK8sClient,
		k8s.ProvideRESTConfig,
		k8s.ProvideClientset,
		k8s.ProvidePortForwardClient,
		k8s.ProvideContainerRuntime,
		k8s.ProvideKubeConfig,
	)

	return nil, nil
}
