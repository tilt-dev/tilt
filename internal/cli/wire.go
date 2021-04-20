// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package cli

import (
	"context"
	"time"

	cliclient "github.com/tilt-dev/tilt/internal/cli/client"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch/fsevent"
	"github.com/tilt-dev/tilt/internal/controllers/core/podlogstream"

	"github.com/google/wire"
	"github.com/jonboulle/clockwork"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/cloud"
	"github.com/tilt-dev/tilt/internal/cloud/cloudurl"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/engine"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/engine/dcwatch"
	"github.com/tilt-dev/tilt/internal/engine/dockerprune"
	"github.com/tilt-dev/tilt/internal/engine/fswatch"
	"github.com/tilt-dev/tilt/internal/engine/k8srollout"
	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/engine/metrics"
	"github.com/tilt-dev/tilt/internal/engine/portforward"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/engine/session"
	"github.com/tilt-dev/tilt/internal/engine/telemetry"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/git"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/openurl"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/internal/tracer"
	"github.com/tilt-dev/tilt/internal/user"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
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
	k8s.ProvideContainerRuntime,
	k8s.ProvideServerVersion,
	k8s.ProvideK8sClient,
	k8s.ProvideOwnerFetcher,
	ProvideKubeContextOverride)

var BaseWireSet = wire.NewSet(
	K8sWireSet,
	tiltfile.WireSet,
	git.ProvideGitRemote,

	docker.SwitchWireSet,

	ProvideDeferredExporter,
	metrics.WireSet,
	user.WireSet,
	dockercompose.NewDockerComposeClient,

	clockwork.NewRealClock,
	engine.DeployerWireSet,
	runtimelog.NewPodLogManager,
	podlogstream.NewController,
	portforward.NewController,
	engine.NewBuildController,
	cmd.WireSet,
	local.NewServerController,
	k8swatch.NewPodWatcher,
	k8swatch.NewServiceWatcher,
	k8swatch.NewEventWatchManager,
	configs.NewConfigsController,
	telemetry.NewController,
	dcwatch.NewEventWatcher,
	runtimelog.NewDockerComposeLogManager,
	cloud.WireSet,
	cloudurl.ProvideAddress,
	k8srollout.NewPodMonitor,
	telemetry.NewStartTracker,
	session.NewController,

	build.ProvideClock,
	provideClock,
	hud.WireSet,
	prompt.WireSet,
	wire.Value(openurl.OpenURL(openurl.BrowserOpen)),

	provideLogActions,
	store.NewStore,
	wire.Bind(new(store.RStore), new(*store.Store)),

	dockerprune.NewDockerPruner,

	provideTiltInfo,
	engine.NewUpper,
	engineanalytics.NewAnalyticsUpdater,
	engineanalytics.ProvideAnalyticsReporter,
	provideUpdateModeFlag,
	fswatch.NewManifestSubscriber,
	fsevent.ProvideWatcherMaker,
	fsevent.ProvideTimerMaker,

	controllers.WireSet,

	provideWebVersion,
	provideWebMode,
	provideWebURL,
	provideWebPort,
	provideWebHost,
	server.WireSet,
	provideAssetServer,

	tracer.NewSpanCollector,
	wire.Bind(new(sdktrace.SpanProcessor), new(*tracer.SpanCollector)),
	wire.Bind(new(tracer.SpanSource), new(*tracer.SpanCollector)),

	dirs.UseTiltDevDir,
	token.GetOrCreateToken,

	buildcontrol.NewKINDLoader,

	wire.Value(feature.MainDefaults),
)

var CLIClientWireSet = wire.NewSet(
	BaseWireSet,
	cliclient.WireSet,
)

var UpWireSet = wire.NewSet(
	BaseWireSet,
	engine.ProvideSubscribers,
)

func wireTiltfileResult(ctx context.Context, analytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (cmdTiltfileResultDeps, error) {
	wire.Build(UpWireSet, newTiltfileResultDeps)
	return cmdTiltfileResultDeps{}, nil
}

func wireDockerPrune(ctx context.Context, analytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (dpDeps, error) {
	wire.Build(UpWireSet, newDPDeps)
	return dpDeps{}, nil
}

func wireCmdUp(ctx context.Context, analytics *analytics.TiltAnalytics, cmdTags engineanalytics.CmdTags, subcommand model.TiltSubcommand) (CmdUpDeps, error) {
	wire.Build(UpWireSet,
		wire.Struct(new(CmdUpDeps), "*"))
	return CmdUpDeps{}, nil
}

type CmdUpDeps struct {
	Upper        engine.Upper
	TiltBuild    model.TiltBuild
	Token        token.Token
	CloudAddress cloudurl.Address
	Store        *store.Store
	Prompt       *prompt.TerminalPrompt
}

func wireCmdCI(ctx context.Context, analytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (CmdCIDeps, error) {
	wire.Build(UpWireSet,
		wire.Value(engineanalytics.CmdTags(map[string]string{})),
		wire.Struct(new(CmdCIDeps), "*"),
	)
	return CmdCIDeps{}, nil
}

type CmdCIDeps struct {
	Upper        engine.Upper
	TiltBuild    model.TiltBuild
	Token        token.Token
	CloudAddress cloudurl.Address
	Store        *store.Store
}

func wireCmdUpdog(ctx context.Context,
	analytics *analytics.TiltAnalytics,
	cmdTags engineanalytics.CmdTags,
	subcommand model.TiltSubcommand,
	objects []ctrlclient.Object) (CmdUpdogDeps, error) {
	wire.Build(BaseWireSet,
		provideUpdogSubscriber,
		provideUpdogCmdSubscribers,
		wire.Struct(new(CmdUpdogDeps), "*"))
	return CmdUpdogDeps{}, nil
}

type CmdUpdogDeps struct {
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
		k8s.ProvideMinikubeClient,
	)
	return "", nil
}

func wireK8sClient(ctx context.Context) (k8s.Client, error) {
	wire.Build(
		K8sWireSet,
		k8s.ProvideMinikubeClient,
	)
	return nil, nil
}

func wireK8sVersion(ctx context.Context) (*version.Info, error) {
	wire.Build(K8sWireSet)
	return nil, nil
}

func wireDockerClusterClient(ctx context.Context) (docker.ClusterClient, error) {
	wire.Build(UpWireSet)
	return nil, nil
}

func wireDockerLocalClient(ctx context.Context) (docker.LocalClient, error) {
	wire.Build(UpWireSet)
	return nil, nil
}

func wireDownDeps(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (DownDeps, error) {
	wire.Build(UpWireSet, ProvideDownDeps)
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

func wireLogsDeps(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (LogsDeps, error) {
	wire.Build(UpWireSet, ProvideLogsDeps)
	return LogsDeps{}, nil
}

type LogsDeps struct {
	url     model.WebURL
	printer *hud.IncrementalPrinter
}

func ProvideLogsDeps(u model.WebURL, p *hud.IncrementalPrinter) LogsDeps {
	return LogsDeps{
		url:     u,
		printer: p,
	}
}

func provideClock() func() time.Time {
	return time.Now
}

type DumpImageDeployRefDeps struct {
	DockerBuilder build.DockerBuilder
	DockerClient  docker.Client
}

func wireDumpImageDeployRefDeps(ctx context.Context) (DumpImageDeployRefDeps, error) {
	wire.Build(UpWireSet,
		wire.Struct(new(DumpImageDeployRefDeps), "*"))
	return DumpImageDeployRefDeps{}, nil
}

func wireAnalytics(l logger.Logger, cmdName model.TiltSubcommand) (*tiltanalytics.TiltAnalytics, error) {
	wire.Build(UpWireSet,
		newAnalytics)
	return nil, nil
}

func wireClientGetter(ctx context.Context) (*cliclient.Getter, error) {
	wire.Build(CLIClientWireSet)
	return nil, nil
}
