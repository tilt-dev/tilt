// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package synclet

import (
	"context"

	"github.com/google/wire"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/minikube"
)

func WireSynclet(ctx context.Context, env k8s.Env, runtime container.Runtime) (*Synclet, error) {
	wire.Build(
		minikube.ProvideMinikubeClient,
		docker.ProvideEnv,
		docker.ProvideDockerClient,
		docker.ProvideDockerVersion,
		docker.DefaultClient,
		wire.Bind(new(docker.Client), new(docker.Cli)),

		NewSynclet,
	)
	return nil, nil
}
