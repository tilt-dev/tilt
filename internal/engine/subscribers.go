package engine

import (
	"github.com/windmilleng/tilt/internal/cloud"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/engine/dockerprune"
	"github.com/windmilleng/tilt/internal/engine/k8swatch"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/store"
)

func ProvideSubscribers(
	hud hud.HeadsUpDisplay,
	pw *k8swatch.PodWatcher,
	sw *k8swatch.ServiceWatcher,
	plm *PodLogManager,
	pfc *PortForwardController,
	fwm *WatchManager,
	bc *BuildController,
	ic *ImageController,
	cc *configs.ConfigsController,
	dcw *DockerComposeEventWatcher,
	dclm *DockerComposeLogManager,
	pm *ProfilerManager,
	sm containerupdate.SyncletManager,
	ar *AnalyticsReporter,
	hudsc *server.HeadsUpServerController,
	tvc *TiltVersionChecker,
	ta *TiltAnalyticsSubscriber,
	ewm *k8swatch.EventWatchManager,
	tcum *cloud.CloudUsernameManager,
	dp *dockerprune.DockerPruner) []store.Subscriber {
	return []store.Subscriber{
		hud,
		pw,
		sw,
		plm,
		pfc,
		fwm,
		bc,
		ic,
		cc,
		dcw,
		dclm,
		pm,
		sm,
		ar,
		hudsc,
		tvc,
		ta,
		ewm,
		tcum,
		dp,
	}
}
