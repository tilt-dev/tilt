// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/go-cloud/wire"
	build "github.com/windmilleng/tilt/internal/build"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	dirs "github.com/windmilleng/wmclient/pkg/dirs"
)

func provideBuildAndDeployer(
	ctx context.Context,
	docker build.DockerClient,
	k8s k8s.Client,
	dir *dirs.WindmillDir,
	env k8s.Env,
	skipContainer bool) (BuildAndDeployer, error) {
	wire.Build(
		// dockerImageBuilder ( = ImageBuilder)
		build.DefaultImageBuilder,
		build.NewDockerImageBuilder,
		build.DefaultConsole,
		build.DefaultOut,
		wire.Value(build.Labels{}),

		NewImageBuildAndDeployer,

		// ContainerBuildAndDeployer ( = BuildAndDeployer)
		wire.Bind(new(BuildAndDeployer), new(ContainerBuildAndDeployer)),
		NewContainerBuildAndDeployer,
		build.NewContainerUpdater,
	)
	return nil, nil
}
