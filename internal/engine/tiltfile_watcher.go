package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type TiltfileController struct {
	disabledForTesting bool
}

func NewTiltfileController() *TiltfileController {
	return &TiltfileController{}
}

func (tc *TiltfileController) DisableForTesting(disabled bool) {
	tc.disabledForTesting = disabled
}

func (tc *TiltfileController) OnChange(ctx context.Context, st store.RStore) {
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
		st.Dispatch(TiltfileReloadStartedAction{FilesChanged: filesChanged})
		t, err := tiltfile.Load(ctx, tiltfile.FileName)
		if err != nil {
			st.Dispatch(TiltfileReloadedAction{
				Err: err,
			})
			return
		}
		manifests, globalYAML, configFiles, err := t.GetManifestConfigsAndGlobalYAML(ctx, initManifests...)
		st.Dispatch(TiltfileReloadedAction{
			Manifests:   manifests,
			GlobalYAML:  globalYAML,
			ConfigFiles: configFiles,
			Err:         err,
		})
	}()
}
