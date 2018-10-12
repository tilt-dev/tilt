// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"

	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

func wireManifestCreator(ctx context.Context, browser engine.BrowserMode) (model.ManifestCreator, error) {
	wire.Build(
		k8s.DetectEnv,

		k8s.ProvidePortForwarder,
		k8s.ProvideRESTClient,
		k8s.ProvideRESTConfig,
		k8s.NewK8sClient,
		wire.Bind(new(k8s.Client), k8s.K8sClient{}),

		docker.DefaultDockerClient,
		wire.Bind(new(docker.DockerClient), new(docker.DockerCli)),

		build.NewImageReaper,

		engine.DeployerWireSet,
		engine.DefaultShouldFallBack,
		engine.ProvidePodWatcherMaker,
		engine.ProvideServiceWatcherMaker,
		engine.NewPodLogManager,

		hud.NewDefaultHeadsUpDisplay,

		engine.NewUpper,
		wire.Bind(new(model.ManifestCreator), engine.Upper{}),
		provideAnalytics,
		provideUpdateModeFlag,
	)
	return nil, nil
}
