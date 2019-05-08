package cli

import (
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
)

func ProvideSubscribers(
	hud hud.HeadsUpDisplay,
	pw *engine.PodWatcher,
	sw *engine.ServiceWatcher,
	plm *engine.PodLogManager,
	pfc *engine.PortForwardController,
	fwm *engine.WatchManager,
	bc *engine.BuildController,
	ic *engine.ImageController,
	cc *engine.ConfigsController,
	dcw *engine.DockerComposeEventWatcher,
	dclm *engine.DockerComposeLogManager,
	pm *engine.ProfilerManager,
	sm engine.SyncletManager,
	ar *engine.AnalyticsReporter,
	hudsc *server.HeadsUpServerController,
	sail client.SailClient) []store.Subscriber {
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
	}
}
