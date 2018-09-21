// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

var DeployerWireSet = wire.NewSet(
	// dockerImageBuilder ( = ImageBuilder)
	build.DefaultConsole,
	build.DefaultOut,
	wire.Value(build.Labels{}),

	build.DefaultImageBuilder,
	build.NewDockerImageBuilder,

	NewSidecarSyncletManager,

	// BuildOrder
	NewImageBuildAndDeployer,
	build.NewContainerUpdater, // in case it's a LocalContainerBuildAndDeployer
	build.NewContainerResolver,
	NewSyncletBuildAndDeployer,
	NewLocalContainerBuildAndDeployer,
	DefaultBuildOrder,

	wire.Bind(new(BuildAndDeployer), new(CompositeBuildAndDeployer)),
	NewCompositeBuildAndDeployer)

func provideBuildAndDeployer(
	ctx context.Context,
	docker docker.DockerClient,
	k8s k8s.Client,
	dir *dirs.WindmillDir,
	env k8s.Env,
	sCli synclet.SyncletClient,
	shouldFallBackToImgBuild FallbackTester) (BuildAndDeployer, error) {
	wire.Build(
		DeployerWireSet,
		analytics.NewMemoryAnalytics,
		wire.Bind(new(analytics.Analytics), new(analytics.MemoryAnalytics)),
	)

	return nil, nil
}
