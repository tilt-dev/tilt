package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
)

type TiltfileWatcher struct {
	tiltfilePath       string
	fsWatcherMaker     FsWatcherMaker
	disabledForTesting bool
	tiltfileWatcher    watch.Notify
}

func NewTiltfileWatcher(watcherMaker FsWatcherMaker) *TiltfileWatcher {
	return &TiltfileWatcher{
		fsWatcherMaker: watcherMaker,
	}
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

	if t.tiltfilePath != state.TiltfilePath || t.tiltfilePath == "" {
		err := t.setupWatch(state.TiltfilePath)
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			return
		}
		go t.watchLoop(ctx, st, initManifests)
	}
}

func (t *TiltfileWatcher) setupWatch(path string) error {
	watcher, err := t.fsWatcherMaker()
	if err != nil {
		return err
	}

	err = watcher.Add(path)
	if err != nil {
		return err
	}

	t.tiltfileWatcher = watcher
	t.tiltfilePath = path

	return nil
}

func (t *TiltfileWatcher) watchLoop(ctx context.Context, st store.RStore, initManifests []model.ManifestName) {
	watcher := t.tiltfileWatcher
	for {
		select {
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			st.Dispatch(NewErrorAction(err))
		case <-ctx.Done():
			return
		case _, ok := <-watcher.Events():
			if !ok {
				return
			}

			manifests, globalYAML, err := getAllManifestsFromTiltfile(ctx, initManifests)
			st.Dispatch(TiltfileReloadedAction{
				Manifests:  manifests,
				GlobalYAML: globalYAML,
				Err:        err,
			})
		}
	}
}
