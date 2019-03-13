package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	clock              func() time.Time
	activeBuild        bool
}

func NewConfigsController(tfl tiltfile.TiltfileLoader) *ConfigsController {
	return &ConfigsController{
		tfl:   tfl,
		clock: time.Now,
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

	go func() {
		startTime := cc.clock()
		st.Dispatch(ConfigsReloadStartedAction{FilesChanged: filesChanged, StartTime: startTime})

		matching := map[string]bool{}
		for _, m := range initManifests {
			matching[string(m)] = true
		}
		tiltfilePath, err := state.RelativeTiltfilePath()
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			return
		}

		tlw := NewTiltfileLogWriter(st)

		manifests, globalYAML, configFiles, warnings, err := cc.tfl.Load(ctx, tiltfilePath, matching, tlw)
		if err == nil && len(manifests) == 0 && globalYAML.Empty() {
			err = fmt.Errorf("No resources found. Check out https://docs.tilt.dev/tutorial.html to get started!")
		}
		if err != nil {
			logger.Get(ctx).Infof(err.Error())
		}
		st.Dispatch(ConfigsReloadedAction{
			Manifests:   manifests,
			GlobalYAML:  globalYAML,
			ConfigFiles: configFiles,
			StartTime:   startTime,
			FinishTime:  cc.clock(),
			Err:         err,
			Warnings:    warnings,
		})
	}()
}
