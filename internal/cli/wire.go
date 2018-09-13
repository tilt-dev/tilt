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
)

func wireServiceCreator(ctx context.Context, browser engine.BrowserMode) (model.ManifestCreator, error) {
	wire.Build(
		k8s.DetectEnv,

		k8s.NewKubectlClient,
		wire.Bind(new(k8s.Client), k8s.KubectlClient{}),

		build.DefaultDockerClient,
		wire.Bind(new(build.DockerClient), new(build.DockerCli)),

		build.NewImageReaper,

		engine.DefaultSyncletClient,
		engine.DeployerWireSet,
		engine.DefaultShouldFallBack,

		engine.NewUpper,
		wire.Bind(new(model.ManifestCreator), engine.Upper{}),
		provideAnalytics,
	)
	return nil, nil
}
