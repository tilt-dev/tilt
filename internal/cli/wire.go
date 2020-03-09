// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/jonboulle/clockwork"
	"github.com/windmilleng/wmclient/pkg/dirs"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/cloud"
	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/engine"
	engineanalytics "github.com/windmilleng/tilt/internal/engine/analytics"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
	"github.com/windmilleng/tilt/internal/engine/k8srollout"
	"github.com/windmilleng/tilt/internal/engine/k8swatch"
	"github.com/windmilleng/tilt/internal/engine/local"
	"github.com/windmilleng/tilt/internal/engine/runtimelog"
	"github.com/windmilleng/tilt/internal/engine/telemetry"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
)

var K8sWireSet = wire.NewSet(
	k8s.ProvideEnv,
	k8s.ProvideClusterName,
	k8s.ProvideKubeContext,
	k8s.ProvideKubeConfig,
	k8s.ProvideClientConfig,
	k8s.ProvideClientset,
	k8s.ProvideRESTConfig,
	k8s.ProvidePortForwardClient,
	k8s.ProvideConfigNamespace,
	k8s.ProvideKubectlRunner,
	k8s.ProvideContainerRuntime,
	k8s.ProvideServerVersion,
	k8s.ProvideK8sClient,
	k8s.ProvideOwnerFetcher)

var BaseWireSet = wire.NewSet(
	K8sWireSet,
	tiltfile.WireSet,
	provideKubectlLogLevel,

	docker.SwitchWireSet,

	dockercompose.NewDockerComposeClient,

	clockwork.NewRealClock,
	engine.DeployerWireSet,
	runtimelog.NewPodLogManager,
	engine.NewPortForwardController,
	engine.NewBuildController,
	local.ProvideExecer,
	local.NewController,
	k8swatch.NewPodWatcher,
	k8swatch.NewServiceWatcher,
	k8swatch.NewEventWatchManager,
	configs.NewConfigsController,
	telemetry.NewController,
	engine.NewDockerComposeEventWatcher,
	runtimelog.NewDockerComposeLogManager,
	engine.NewProfilerManager,
	engine.NewGithubClientFactory,
	engine.NewTiltVersionChecker,
	cloud.WireSet,
	cloudurl.ProvideAddress,
	k8srollout.NewPodMonitor,
	telemetry.NewStartTracker,

	provideClock,
	hud.WireSet,

	provideLogActions,
	store.NewStore,
	wire.Bind(new(store.RStore), new(*store.Store)),

	dockerprune.NewDockerPruner,

	provideTiltInfo,
	engine.ProvideSubscribers,
	engine.NewUpper,
	engineanalytics.NewAnalyticsUpdater,
	engineanalytics.ProvideAnalyticsReporter,
	provideUpdateModeFlag,
	engine.NewWatchManager,
	engine.ProvideFsWatcherMaker,
	engine.ProvideTimerMaker,

	provideWebVersion,
	provideWebMode,
	provideWebURL,
	provideWebPort,
	provideWebHost,
	provideNoBrowserFlag,
	server.ProvideHeadsUpServer,
	provideAssetServer,
	server.ProvideHeadsUpServerController,

	tracer.NewSpanCollector,
	wire.Bind(new(sdktrace.SpanProcessor), new(*tracer.SpanCollector)),
	wire.Bind(new(tracer.SpanSource), new(*tracer.SpanCollector)),

	dirs.UseWindmillDir,
	token.GetOrCreateToken,

	wire.Struct(new(CmdUpDeps), "*"),
	engine.NewKINDLoader,

	wire.Value(feature.MainDefaults),
)

func wireTiltfileResult(ctx context.Context, analytics *analytics.TiltAnalytics) (cmdTiltfileResultDeps, error) {
	wire.Build(BaseWireSet, newTiltfileResultDeps)
	return cmdTiltfileResultDeps{}, nil
}

func wireDockerPrune(ctx context.Context, analytics *analytics.TiltAnalytics) (dpDeps, error) {
	wire.Build(BaseWireSet, newDPDeps)
	return dpDeps{}, nil
}

func wireCmdUp(ctx context.Context, hudEnabled hud.HudEnabled, analytics *analytics.TiltAnalytics, cmdUpTags engineanalytics.CmdUpTags) (CmdUpDeps, error) {
	wire.Build(BaseWireSet, build.ProvideClock)
	return CmdUpDeps{}, nil
}

type CmdUpDeps struct {
	Hud          hud.HeadsUpDisplay
	Upper        engine.Upper
	TiltBuild    model.TiltBuild
	Token        token.Token
	CloudAddress cloudurl.Address
	Store        *store.Store
}

func wireKubeContext(ctx context.Context) (k8s.KubeContext, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireKubeConfig(ctx context.Context) (*api.Config, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func wireEnv(ctx context.Context) (k8s.Env, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireNamespace(ctx context.Context) (k8s.Namespace, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireClusterName(ctx context.Context) (k8s.ClusterName, error) {
	wire.Build(K8sWireSet)
	return "", nil
}

func wireRuntime(ctx context.Context) (container.Runtime, error) {
	wire.Build(
		K8sWireSet,
		provideKubectlLogLevel,
	)
	return "", nil
}

func wireK8sVersion(ctx context.Context) (*version.Info, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func wireDockerClusterClient(ctx context.Context) (docker.ClusterClient, error) {
	wire.Build(BaseWireSet)
	return nil, nil
}

func wireDockerLocalClient(ctx context.Context) (docker.LocalClient, error) {
	wire.Build(BaseWireSet)
	return nil, nil
}

func wireDownDeps(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics) (DownDeps, error) {
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
