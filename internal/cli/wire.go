// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"
	"time"

	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/demo"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
)

var K8sWireSet = wire.NewSet(
	k8s.DetectEnv,
	k8s.DetectNodeIP,

	k8s.ProvidePortForwarder,
	k8s.ProvideRESTClient,
	k8s.ProvideRESTConfig,
	k8s.NewK8sClient,
	wire.Bind(new(k8s.Client), k8s.K8sClient{}))

var BaseWireSet = wire.NewSet(
	K8sWireSet,

	docker.DefaultDockerClient,
	wire.Bind(new(docker.DockerClient), new(docker.DockerCli)),

	build.NewImageReaper,

	engine.DeployerWireSet,
	engine.NewPodLogManager,
	engine.NewPortForwardController,
	engine.NewBuildController,
	engine.NewPodWatcher,
	engine.NewServiceWatcher,
	engine.NewImageController,
	engine.NewConfigsController,
	engine.NewDockerComposeEventWatcher,
	engine.NewDockerComposeLogManager,

	provideClock,
	hud.NewRenderer,
	hud.NewDefaultHeadsUpDisplay,

	provideLogActions,
	store.NewStore,

	engine.NewUpper,
	provideAnalytics,
	provideUpdateModeFlag,
	engine.NewWatchManager,
	engine.ProvideFsWatcherMaker,
	engine.ProvideTimerMaker,

	provideHudAndUpper,
)

func wireDemo(ctx context.Context, branch demo.RepoBranch) (demo.Script, error) {
	wire.Build(BaseWireSet, demo.NewScript)
	return demo.Script{}, nil
}

func wireHudAndUpper(ctx context.Context) (HudAndUpper, error) {
	wire.Build(BaseWireSet)
	return HudAndUpper{}, nil
}

type HudAndUpper struct {
	hud   hud.HeadsUpDisplay
	upper engine.Upper
}

func provideHudAndUpper(h hud.HeadsUpDisplay, upper engine.Upper) HudAndUpper {
	return HudAndUpper{h, upper}
}

func wireK8sClient(ctx context.Context) (k8s.Client, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func provideClock() func() time.Time {
	return time.Now
}
