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

func (t *TiltfileWatcher) OnChange(ctx context.Context, dsr store.DispatchingStateReader) {
	if t.disabledForTesting {
		return
	}
	state := dsr.RLockState()
	defer dsr.RUnlockState()
	initManifests := state.InitManifests

	if t.tiltfilePath != state.TiltfilePath || t.tiltfilePath == "" {
		err := t.setupWatch(state.TiltfilePath)
		if err != nil {
			dsr.Dispatch(NewErrorAction(err))
			return
		}
		go t.watchLoop(ctx, dsr, initManifests)
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

func (t *TiltfileWatcher) watchLoop(ctx context.Context, d store.Dispatcher, initManifests []model.ManifestName) {
	watcher := t.tiltfileWatcher
	for {
		select {
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			d.Dispatch(NewErrorAction(err))
		case <-ctx.Done():
			return
		case _, ok := <-watcher.Events():
			if !ok {
				return
			}

			manifests, globalYAML, err := getNewManifestsFromTiltfile(ctx, initManifests)
			d.Dispatch(TiltfileReloadedAction{
				Manifests:  manifests,
				GlobalYAML: globalYAML,
				Err:        err,
			})
		}
	}
}
