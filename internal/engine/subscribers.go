package engine

import (
	"github.com/tilt-dev/tilt/internal/cloud"
	"github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/engine/apiserver"
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
	hud hud.HeadsUpDisplay,
	ts *hud.TerminalStream,
	tp *prompt.TerminalPrompt,
	pw *k8swatch.PodWatcher,
	sw *k8swatch.ServiceWatcher,
	plm *runtimelog.PodLogManager,
	pfc *portforward.Controller,
	fwm *fswatch.WatchManager,
	gm *fswatch.GitManager,
	bc *BuildController,
	cc *configs.ConfigsController,
	dcw *dcwatch.EventWatcher,
	dclm *runtimelog.DockerComposeLogManager,
	pm *ProfilerManager,
	ar *analytics.AnalyticsReporter,
	hudsc *server.HeadsUpServerController,
	au *analytics.AnalyticsUpdater,
	ewm *k8swatch.EventWatchManager,
	tcum *cloud.CloudStatusManager,
	dp *dockerprune.DockerPruner,
	tc *telemetry.Controller,
	lc *local.Controller,
	podm *k8srollout.PodMonitor,
	ec *exit.Controller,
	mc *metrics.Controller,
	mmc *metrics.ModeController,
	ac *apiserver.Controller,
) []store.Subscriber {
	return []store.Subscriber{
		hud,
		ts,
		tp,
		pw,
		sw,
		plm,
		pfc,
		fwm,
		gm,
		bc,
		cc,
		dcw,
		dclm,
		pm,
		ar,
		hudsc,
		au,
		ewm,
		tcum,
		dp,
		tc,
		lc,
		podm,
		ec,
		mc,
		mmc,
		ac,
	}
}
