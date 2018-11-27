package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type ConfigsController struct {
	disabledForTesting bool
}

func NewConfigsController() *ConfigsController {
	return &ConfigsController{}
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
		manifests, globalYAML, configFiles, err := t.GetManifestConfigsAndGlobalYAML(ctx)
		st.Dispatch(ConfigsReloadedAction{
			Manifests:   manifests,
			GlobalYAML:  globalYAML,
			ConfigFiles: configFiles,
			Err:         err,
		})
	}()
}
