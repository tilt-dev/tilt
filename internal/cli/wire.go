// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"

	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/image"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
	dirs "github.com/windmilleng/wmclient/pkg/dirs"
)

func wireServiceCreator(ctx context.Context, browser engine.BrowserMode) (model.ServiceCreator, error) {
	wire.Build(
		service.ProvideMemoryManager,
		k8s.DetectEnv,

		k8s.NewKubectlClient,
		wire.Bind(new(k8s.Client), k8s.KubectlClient{}),

		dirs.UseWindmillDir,
		image.NewImageHistory,

		build.DefaultDockerClient,
		wire.Bind(new(build.DockerClient), new(build.DockerCli)),

		// dockerImageBuilder ( = ImageBuilder)
		build.DefaultConsole,
		build.DefaultOut,
		build.DefaultImageBuilder,
		build.NewDockerImageBuilder,
		engine.NewImageBuildAndDeployer,

		// ContainerBuildAndDeployer ( = BuildAndDeployer)
		wire.Bind(new(engine.BuildAndDeployer), new(engine.ContainerBuildAndDeployer)),
		engine.NewContainerBuildAndDeployer,
		build.NewContainerUpdater,

		engine.NewUpper,
		provideServiceCreator,
	)
	return nil, nil
}
