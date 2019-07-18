package engine

import (
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
)

func ProvideSubscribers(
	hud hud.HeadsUpDisplay,
	pw *PodWatcher,
	sw *ServiceWatcher,
	plm *PodLogManager,
	pfc *PortForwardController,
	fwm *WatchManager,
	bc *BuildController,
	ic *ImageController,
	cc *ConfigsController,
	dcw *DockerComposeEventWatcher,
	dclm *DockerComposeLogManager,
	pm *ProfilerManager,
	sm containerupdate.SyncletManager,
	ar *AnalyticsReporter,
	hudsc *server.HeadsUpServerController,
	sail client.SailClient,
	tvc *TiltVersionChecker,
	ta *TiltAnalyticsSubscriber,
	ewm *EventWatchManager) []store.Subscriber {
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
		sail,
		tvc,
		ta,
		ewm,
	}
}
