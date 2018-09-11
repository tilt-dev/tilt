// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"

	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
)

func wireServiceCreator(ctx context.Context, browser engine.BrowserMode) (model.ServiceCreator, error) {
	wire.Build(
		service.ProvideMemoryManager,
		k8s.DetectEnv,

		k8s.NewKubectlClient,
		wire.Bind(new(k8s.Client), k8s.KubectlClient{}),

		build.DefaultDockerClient,
		wire.Bind(new(build.DockerClient), new(build.DockerCli)),

		// dockerImageBuilder ( = ImageBuilder)
		build.DefaultConsole,
		build.DefaultOut,
		build.DefaultImageBuilder,
		build.NewDockerImageBuilder,
		wire.Value(build.Labels{}),
		build.NewImageReaper,

		// ImageBuildAndDeployer (FallbackBuildAndDeployer)
		wire.Bind(new(engine.FallbackBuildAndDeployer), new(engine.ImageBuildAndDeployer)),
		engine.NewImageBuildAndDeployer,

		// FirstLineBuildAndDeployer (LocalContainerBaD OR SyncletBaD)
		build.NewContainerUpdater, // in case it's a LocalContainerBuildAndDeployer
		engine.NewFirstLineBuildAndDeployer,

		wire.Bind(new(engine.BuildAndDeployer), new(engine.CompositeBuildAndDeployer)),
		engine.DefaultShouldFallBack,
		engine.NewCompositeBuildAndDeployer,

		engine.NewUpper,
		provideServiceCreator,
	)
	return nil, nil
}
