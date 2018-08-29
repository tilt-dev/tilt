// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/go-cloud/wire"
	build "github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	dirs "github.com/windmilleng/wmclient/pkg/dirs"
)

func provideBuildAndDeployer(
	ctx context.Context,
	docker build.DockerClient,
	k8s k8s.Client,
	dir *dirs.WindmillDir,
	env k8s.Env) (BuildAndDeployer, error) {
	wire.Build(
		image.NewImageHistory,
		build.DefaultImageBuilder,
		build.NewLocalDockerBuilder,
		NewLocalBuildAndDeployer,
		build.DefaultConsole,
		build.DefaultOut)
	return nil, nil
}
