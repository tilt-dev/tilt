// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/tiltfile"

	"github.com/docker/docker/api/types"
	"github.com/google/wire"
	"k8s.io/apimachinery/pkg/version"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
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
	k8s.ProvideEnv,
	k8s.DetectNodeIP,
	k8s.ProvideKubeContext,
	k8s.ProvideClientConfig,
	k8s.ProvideClientSet,
	k8s.ProvideRESTConfig,
	k8s.ProvidePortForwarder,
	k8s.ProvideConfigNamespace,
	k8s.ProvideKubectlRunner,
	k8s.ProvideContainerRuntime,
	k8s.ProvideServerVersion,
	k8s.ProvideK8sClient)

var BaseWireSet = wire.NewSet(
	K8sWireSet,

	docker.ProvideDockerClient,
	docker.ProvideDockerVersion,
	docker.DefaultClient,
	wire.Bind(new(docker.Client), new(docker.Cli)),

	dockercompose.NewDockerComposeClient,

	build.NewImageReaper,

	tiltfile.ProvideTiltfileLoader,

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
	engine.NewProfilerManager,

	provideClock,
	hud.NewRenderer,
	hud.NewDefaultHeadsUpDisplay,

	provideLogActions,
	store.NewStore,
	wire.Bind(new(store.RStore), new(store.Store)),

	engine.NewUpper,
	provideAnalytics,
	engine.ProvideAnalyticsReporter,
	provideUpdateModeFlag,
	engine.NewWatchManager,
	engine.ProvideFsWatcherMaker,
	engine.ProvideTimerMaker,

	provideWebVersion,
	provideWebMode,
	provideWebURL,
	provideWebPort,
	server.ProvideHeadsUpServer,
	server.ProvideAssetServer,
	server.ProvideHeadsUpServerController,

	provideThreads,
)

func wireDemo(ctx context.Context, branch demo.RepoBranch) (demo.Script, error) {
	wire.Build(BaseWireSet, demo.NewScript, build.ProvideClock)
	return demo.Script{}, nil
}

func wireThreads(ctx context.Context) (Threads, error) {
	wire.Build(BaseWireSet, build.ProvideClock)
	return Threads{}, nil
}

type Threads struct {
	hud   hud.HeadsUpDisplay
	upper engine.Upper
}

func provideThreads(h hud.HeadsUpDisplay, upper engine.Upper) Threads {
	return Threads{h, upper}
}

func wireK8sClient(ctx context.Context) (k8s.Client, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func wireKubeContext(ctx context.Context) (k8s.KubeContext, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireEnv(ctx context.Context) (k8s.Env, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireNamespace(ctx context.Context) (k8s.Namespace, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireRuntime(ctx context.Context) (container.Runtime, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireK8sVersion(ctx context.Context) (*version.Info, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func wireDockerVersion(ctx context.Context) (types.Version, error) {
	wire.Build(BaseWireSet)
	return types.Version{}, nil
}

func wireDockerEnv(ctx context.Context) (docker.Env, error) {
	wire.Build(BaseWireSet)
	return docker.Env{}, nil
}

func wireDownDeps(ctx context.Context) (DownDeps, error) {
	wire.Build(BaseWireSet, ProvideDownDeps)
	return DownDeps{}, nil
}

type DownDeps struct {
	tfl      tiltfile.TiltfileLoader
	dcClient dockercompose.DockerComposeClient
	kClient  k8s.Client
}

func ProvideDownDeps(
	tfl tiltfile.TiltfileLoader,
	dcClient dockercompose.DockerComposeClient,
	kClient k8s.Client) DownDeps {
	return DownDeps{
		tfl:      tfl,
		dcClient: dcClient,
		kClient:  kClient,
	}
}

func provideClock() func() time.Time {
	return time.Now
}
