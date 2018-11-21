package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type ConfigsController struct {
	disabledForTesting bool
}

func NewTiltfileController() *ConfigsController {
	return &ConfigsController{}
}

func (tc *ConfigsController) DisableForTesting(disabled bool) {
	tc.disabledForTesting = disabled
}

func (tc *ConfigsController) OnChange(ctx context.Context, st store.RStore) {
	if tc.disabledForTesting {
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
		st.Dispatch(ConfigsReloadStartedAction{FilesChanged: filesChanged})
		t, err := tiltfile.Load(ctx, tiltfile.FileName)
		if err != nil {
			st.Dispatch(ConfigsReloadedAction{
				Err: err,
			})
			return
		}
		manifests, globalYAML, configFiles, err := t.GetManifestConfigsAndGlobalYAML(ctx, initManifests...)
		st.Dispatch(ConfigsReloadedAction{
			Manifests:   manifests,
			GlobalYAML:  globalYAML,
			ConfigFiles: configFiles,
			Err:         err,
		})
	}()
}
