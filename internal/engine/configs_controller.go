package engine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	clock              func() time.Time
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

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore) {
	if cc.disabledForTesting {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()
	initManifests := state.InitManifests
	if len(state.PendingConfigFileChanges) == 0 {
		return
	}

	filesChanged := make(map[string]bool)
	for k, v := range state.PendingConfigFileChanges {
		filesChanged[k] = v
	}
	// TODO(dbentley): there's a race condition where we start it before we clear it, so we could start many tiltfile reloads...
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

		logWriter := logger.Get(ctx).Writer(logger.InfoLvl)
		prefix := fmt.Sprintf("[%s] ", tiltfile.FileName)
		prefixLogWriter := logger.NewPrefixedWriter(prefix, logWriter)
		actionWriter := NewTiltfileLogWriter(st)
		multiWriter := io.MultiWriter(prefixLogWriter, actionWriter)

		loadCtx := logger.WithLogger(ctx, logger.NewLogger(logger.Get(ctx).Level(), multiWriter))

		manifests, globalYAML, configFiles, warnings, err := cc.tfl.Load(loadCtx, tiltfilePath, matching)
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
