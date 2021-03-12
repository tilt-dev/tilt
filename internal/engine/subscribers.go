package engine

import (
	"github.com/tilt-dev/tilt/internal/cloud"
	"github.com/tilt-dev/tilt/internal/controllers"
	"github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/engine/dcwatch"
	"github.com/tilt-dev/tilt/internal/engine/dockerprune"
	"github.com/tilt-dev/tilt/internal/engine/exit"
	"github.com/tilt-dev/tilt/internal/engine/fswatch"
	"github.com/tilt-dev/tilt/internal/engine/k8srollout"
	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/engine/metrics"
	"github.com/tilt-dev/tilt/internal/engine/portforward"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/engine/telemetry"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
)

func ProvideSubscribers(
	hudsc *server.HeadsUpServerController,
	tscm *controllers.TiltServerControllerManager,
	cb *controllers.ControllerBuilder,
	hud hud.HeadsUpDisplay,
	ts *hud.TerminalStream,
	tp *prompt.TerminalPrompt,
	pw *k8swatch.PodWatcher,
	sw *k8swatch.ServiceWatcher,
	plm *runtimelog.PodLogManager,
	pfc *portforward.Controller,
	fsms *fswatch.ManifestSubscriber,
	fwm *fswatch.WatchManager,
	bc *BuildController,
	cc *configs.ConfigsController,
	dcw *dcwatch.EventWatcher,
	dclm *runtimelog.DockerComposeLogManager,
	ar *analytics.AnalyticsReporter,
	au *analytics.AnalyticsUpdater,
	ewm *k8swatch.EventWatchManager,
	tcum *cloud.CloudStatusManager,
	dp *dockerprune.DockerPruner,
	tc *telemetry.Controller,
	lc *local.Controller,
	lsc *local.ServerController,
	podm *k8srollout.PodMonitor,
	ec *exit.Controller,
	mc *metrics.Controller,
	mmc *metrics.ModeController,
) []store.Subscriber {
	return []store.Subscriber{
		// The API server must go before other subscribers,
		// so that it can run its boot sequence first.
		hudsc,
		// The controller manager must go after the API server,
		// so that it can connect to it and make resources available.
		tscm,
		cb,

		hud,
		ts,
		tp,
		pw,
		sw,
		plm,
		pfc,
		fsms,
		fwm,
		bc,
		cc,
		dcw,
		dclm,
		ar,
		au,
		ewm,
		tcum,
		dp,
		tc,
		lsc,
		podm,
		ec,
		mc,
		mmc,
	}
}
