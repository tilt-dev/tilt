// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

var DeployerBaseWireSet = wire.NewSet(
	// dockerImageBuilder ( = ImageBuilder)
	wire.Value(dockerfile.Labels{}),
	wire.Value(UpperReducer),

	build.DefaultImageBuilder,
	build.NewCacheBuilder,
	build.NewDockerImageBuilder,

	// BuildOrder
	NewImageBuildAndDeployer,
	build.NewContainerUpdater, // in case it's a LocalContainerBuildAndDeployer
	NewSyncletBuildAndDeployer,
	NewLocalContainerBuildAndDeployer,
	NewDockerComposeBuildAndDeployer,
	NewImageAndCacheBuilder,
	DefaultBuildOrder,

	wire.Bind(new(BuildAndDeployer), new(CompositeBuildAndDeployer)),
	NewCompositeBuildAndDeployer,
	ProvideUpdateMode,
	NewGlobalYAMLBuildController,
)

var DeployerWireSetTest = wire.NewSet(
	DeployerBaseWireSet,
	NewSyncletManagerForTests,
)

var DeployerWireSet = wire.NewSet(
	DeployerBaseWireSet,
	NewSyncletManager,
)

func provideBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	dir *dirs.WindmillDir,
	env k8s.Env,
	updateMode UpdateModeFlag,
	sCli synclet.SyncletClient,
	dcc dockercompose.DockerComposeClient) (BuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		analytics.NewMemoryAnalytics,
		wire.Bind(new(analytics.Analytics), new(analytics.MemoryAnalytics)),
		build.ProvideClock,
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}

func provideImageBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	dir *dirs.WindmillDir) (*ImageBuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		analytics.NewMemoryAnalytics,
		wire.Bind(new(analytics.Analytics), new(analytics.MemoryAnalytics)),
		wire.Value(k8s.Env(k8s.EnvDockerDesktop)),
		wire.Value(UpdateModeFlag(UpdateModeAuto)),
		build.ProvideClock,
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}

func provideDockerComposeBuildAndDeployer(
	ctx context.Context,
	dcCli dockercompose.DockerComposeClient,
	dCli docker.Client,
	dir *dirs.WindmillDir) (*DockerComposeBuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		wire.Value(UpdateModeFlag(UpdateModeAuto)),
		build.ProvideClock,

		// EnvNone ensures that we get an exploding k8s client.
		wire.Value(k8s.Env(k8s.EnvNone)),
		k8s.ProvideClientConfig,
		k8s.ProvideConfigNamespace,
		k8s.ProvideKubeContext,
		k8s.ProvideKubectlRunner,
		k8s.ProvideK8sClient,
		k8s.ProvidePortForwarder,
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}
