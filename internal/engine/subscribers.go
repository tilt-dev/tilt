package engine

import (
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
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
	gybc *GlobalYAMLBuildController,
	cc *ConfigsController,
	dcw *DockerComposeEventWatcher,
	dclm *DockerComposeLogManager,
	pm *ProfilerManager,
	sm SyncletManager,
	ar *AnalyticsReporter,
	hudsc *server.HeadsUpServerController) []store.Subscriber {
	return []store.Subscriber{
		hud,
		pw,
		sw,
		plm,
		pfc,
		fwm,
		bc,
		ic,
		gybc,
		cc,
		dcw,
		dclm,
		pm,
		sm,
		ar,
		hudsc,
	}
}
