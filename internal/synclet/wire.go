// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package synclet

import (
	"context"

	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/wmdocker"
)

func WireSynclet(ctx context.Context, env k8s.Env) (*Synclet, error) {
	wire.Build(
		wmdocker.DefaultDockerClient,
		wire.Bind(new(wmdocker.DockerClient), new(wmdocker.DockerCli)),

		NewSynclet,
	)
	return nil, nil
}
