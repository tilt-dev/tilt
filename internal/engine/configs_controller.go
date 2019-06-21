package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	dockerClient       docker.Client
	clock              func() time.Time
}

func NewConfigsController(tfl tiltfile.TiltfileLoader, dockerClient docker.Client) *ConfigsController {
	return &ConfigsController{
		tfl:          tfl,
		dockerClient: dockerClient,
		clock:        time.Now,
	}
}

func (cc *ConfigsController) DisableForTesting(disabled bool) {
	cc.disabledForTesting = disabled
}

// Modeled after BuildController.nextTargetToBuild. Check to see that:
// 1) There's currently no Tiltfile build running,
// 2) There are pending file changes, and
// 3) Those files have changed since the last Tiltfile build
//    (so that we don't keep re-running a failed build)
func (cc *ConfigsController) shouldBuild(state store.EngineState) bool {
	isRunning := !state.CurrentTiltfileBuild.StartTime.IsZero()
	if isRunning {
		return false
	}

	for _, changeTime := range state.PendingConfigFileChanges {
		lastStartTime := state.LastTiltfileBuild.StartTime
		if changeTime.After(lastStartTime) {
			return true
		}
	}
	return false
}

func (cc *ConfigsController) loadTiltfile(ctx context.Context, st store.RStore,
	initManifests []model.ManifestName, filesChanged map[string]bool, tiltfilePath string) {

	startTime := cc.clock()
	st.Dispatch(ConfigsReloadStartedAction{FilesChanged: filesChanged, StartTime: startTime})

	matching := map[string]bool{}
	for _, m := range initManifests {
		matching[string(m)] = true
	}

	actionWriter := NewTiltfileLogWriter(st)
	loadCtx := logger.WithLogger(ctx, logger.NewLogger(logger.Get(ctx).Level(), actionWriter))

	tlr, err := cc.tfl.Load(loadCtx, tiltfilePath, matching)
	if err == nil && len(tlr.Manifests) == 0 {
		err = fmt.Errorf("No resources found. Check out https://docs.tilt.dev/tutorial.html to get started!")
	}
	if err != nil {
		logger.Get(loadCtx).Infof(err.Error())
	}

	if tlr.Orchestrator() != model.OrchestratorUnknown {
		cc.dockerClient.SetOrchestrator(tlr.Orchestrator())
	}

	st.Dispatch(ConfigsReloadedAction{
		Manifests:          tlr.Manifests,
		ConfigFiles:        tlr.ConfigFiles,
		TiltIgnoreContents: tlr.TiltIgnoreContents,
		StartTime:          startTime,
		FinishTime:         cc.clock(),
		Err:                err,
		Warnings:           tlr.Warnings,
	})
}

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore) {
	if cc.disabledForTesting {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	initManifests := state.InitManifests
	if !cc.shouldBuild(state) {
		return
	}

	filesChanged := make(map[string]bool)
	for k := range state.PendingConfigFileChanges {
		filesChanged[k] = true
	}

	tiltfilePath, err := state.RelativeTiltfilePath()
	if err != nil {
		st.Dispatch(NewErrorAction(err))
		return
	}

	// Release the state lock and load the tiltfile in a separate goroutine
	go cc.loadTiltfile(ctx, st, initManifests, filesChanged, tiltfilePath)
}
