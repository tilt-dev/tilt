package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type TiltfileWatcher struct {
	disabledForTesting bool
}

func NewTiltfileWatcher(watcherMaker FsWatcherMaker) *TiltfileWatcher {
	return &TiltfileWatcher{}
}

func (t *TiltfileWatcher) DisableForTesting(disabled bool) {
	t.disabledForTesting = disabled
}

func (t *TiltfileWatcher) OnChange(ctx context.Context, st store.RStore) {
	if t.disabledForTesting {
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

// func (t *TiltfileWatcher) setupWatch(path string) error {
// 	if t.tiltfileWatcher != nil {
// 		t.cancelChan <- struct{}{}
// 	}
// 	watcher, err := t.fsWatcherMaker()
// 	if err != nil {
// 		return err
// 	}

// 	err = watcher.Add(path)
// 	if err != nil {
// 		return err
// 	}

// 	t.tiltfileWatcher = watcher
// 	t.tiltfilePath = path

// 	return nil
// }

// func (t *TiltfileWatcher) watchLoop(ctx context.Context, st store.RStore, initManifests []model.ManifestName) {
// 	watcher := t.tiltfileWatcher
// 	for {
// 		select {
// 		case err, ok := <-watcher.Errors():
// 			if !ok {
// 				return
// 			}
// 			st.Dispatch(NewErrorAction(err))
// 		case <-ctx.Done():
// 			return
// 		case <-t.cancelChan:
// 			return
// 		case _, ok := <-watcher.Events():
// 			if !ok {
// 				return
// 			}

// 			manifests, globalYAML, configWatches, err := getNewManifestsFromTiltfile(ctx, initManifests)
// 			st.Dispatch(TiltfileReloadedAction{
// 				Manifests:     manifests,
// 				GlobalYAML:    globalYAML,
// 				ConfigWatches: configWatches,
// 				Err:           err,
// 			})
// 		}
// 	}
// }
