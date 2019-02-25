package engine

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	clock              func() time.Time
	analytics          analytics.Analytics
}

func NewConfigsController(tfl tiltfile.TiltfileLoader, analytics analytics.Analytics) *ConfigsController {
	return &ConfigsController{
		tfl:       tfl,
		clock:     time.Now,
		analytics: analytics,
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

		tlw := NewTiltfileLogWriter(st)

		manifests, globalYAML, configFiles, warnings, builtinCallCounts, err := cc.tfl.Load(ctx, tiltfilePath, matching, tlw)
		if err == nil && len(manifests) == 0 && globalYAML.Empty() {
			err = fmt.Errorf("No resources found. Check out https://docs.tilt.dev/tutorial.html to get started!")
		}
		if err != nil {
			logger.Get(ctx).Infof(err.Error())
		}

		cc.reportTiltfileLoaded(builtinCallCounts)
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

func (cc *ConfigsController) reportTiltfileLoaded(counts map[string]int) {
	tags := make(map[string]string)
	for builtinName, count := range counts {
		tags[fmt.Sprintf("tiltfile.invoked.%s", builtinName)] = strconv.Itoa(count)
	}
	cc.analytics.Incr("tiltfile.loaded", tags)
}
