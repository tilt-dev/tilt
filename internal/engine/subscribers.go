package engine

import (
	"github.com/windmilleng/tilt/internal/cloud"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/engine/analytics"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
	"github.com/windmilleng/tilt/internal/engine/k8srollout"
	"github.com/windmilleng/tilt/internal/engine/k8swatch"
	"github.com/windmilleng/tilt/internal/engine/local"
	"github.com/windmilleng/tilt/internal/engine/runtimelog"
	"github.com/windmilleng/tilt/internal/engine/telemetry"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/store"
)

func ProvideSubscribers(
	hud hud.HeadsUpDisplay,
	pw *k8swatch.PodWatcher,
	sw *k8swatch.ServiceWatcher,
	plm *runtimelog.PodLogManager,
	pfc *PortForwardController,
	fwm *WatchManager,
	bc *BuildController,
	cc *configs.ConfigsController,
	dcw *DockerComposeEventWatcher,
	dclm *runtimelog.DockerComposeLogManager,
	pm *ProfilerManager,
	sm containerupdate.SyncletManager,
	ar *analytics.AnalyticsReporter,
	hudsc *server.HeadsUpServerController,
	tvc *TiltVersionChecker,
	au *analytics.AnalyticsUpdater,
	ewm *k8swatch.EventWatchManager,
	tcum *cloud.CloudUsernameManager,
	cuu *cloud.UpdateUploader,
	dp *dockerprune.DockerPruner,
	tc *telemetry.Controller,
	lc *local.Controller,
	podm *k8srollout.PodMonitor,
) []store.Subscriber {
	return []store.Subscriber{
		hud,
		pw,
		sw,
		plm,
		pfc,
		fwm,
		bc,
		cc,
		dcw,
		dclm,
		pm,
		sm,
		ar,
		hudsc,
		tvc,
		au,
		ewm,
		tcum,
		cuu,
		dp,
		tc,
		lc,
		podm,
	}
}
