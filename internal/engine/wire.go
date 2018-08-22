// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/go-cloud/wire"
	build "github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	service "github.com/windmilleng/tilt/internal/service"
	dirs "github.com/windmilleng/wmclient/pkg/dirs"
)

func ProvideUpperForTesting(ctx context.Context, dir *dirs.WindmillDir, env k8s.Env) (Upper, error) {
	wire.Build(
		service.ProvideMemoryManager,
		image.NewImageHistory,
		build.DefaultDockerClient,
		NewUpper,
		NewLocalBuildAndDeployer)
	return Upper{}, nil
}
