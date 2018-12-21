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
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
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

	dockercompose.NewDockerComposeClient,

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
	wire.Bind(new(store.RStore), new(store.Store)),

	engine.NewUpper,
	provideAnalytics,
	provideUpdateModeFlag,
	engine.NewWatchManager,
	engine.ProvideFsWatcherMaker,
	engine.ProvideTimerMaker,

	server.ProvideHeadsUpServer,

	provideThreads,
)

func wireDemo(ctx context.Context, branch demo.RepoBranch) (demo.Script, error) {
	wire.Build(BaseWireSet, demo.NewScript)
	return demo.Script{}, nil
}

func wireThreads(ctx context.Context) (Threads, error) {
	wire.Build(BaseWireSet)
	return Threads{}, nil
}

type Threads struct {
	hud    hud.HeadsUpDisplay
	upper  engine.Upper
	server server.HeadsUpServer
}

func provideThreads(h hud.HeadsUpDisplay, upper engine.Upper, server server.HeadsUpServer) Threads {
	return Threads{h, upper, server}
}

func wireK8sClient(ctx context.Context) (k8s.Client, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func provideClock() func() time.Time {
	return time.Now
}
