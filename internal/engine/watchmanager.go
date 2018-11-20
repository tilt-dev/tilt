package engine

import (
	"context"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
)

type WatchableManifest interface {
	Dependencies() []string
	ManifestName() model.ManifestName
	ConfigMatcher() (model.PathMatcher, error)
	LocalRepos() []model.LocalGithubRepo
}

// configManifest makes a WatchableManifest that works just for the config files (Tiltfile, yaml, Dockerfiles, etc.)
type tiltfileManifest struct {
	dependencies []string
}

func (m *tiltfileManifest) Dependencies() []string {
	return m.dependencies
}

func (m *tiltfileManifest) ManifestName() model.ManifestName {
	return "Tiltfile"
}

func (m *tiltfileManifest) ConfigMatcher() (model.PathMatcher, error) {
	return model.EmptyMatcher, nil
}

func (m *tiltfileManifest) LocalRepos() []model.LocalGithubRepo {
	return nil
}

type manifestFilesChangedAction struct {
	manifestName model.ManifestName
	files        []string
}

func (manifestFilesChangedAction) Action() {}

type configFilesChangedAction struct {
	files []string
}

func (configFilesChangedAction) Action() {}

type manifestNotifyCancel struct {
	manifest WatchableManifest
	notify   watch.Notify
	cancel   func()
}

type WatchManager struct {
	manifestWatches    map[model.ManifestName]manifestNotifyCancel
	fsWatcherMaker     FsWatcherMaker
	timerMaker         timerMaker
	tiltfileWatch      watch.Notify
	disabledForTesting bool
}

func NewWatchManager(watcherMaker FsWatcherMaker, timerMaker timerMaker) *WatchManager {
	return &WatchManager{
		manifestWatches: make(map[model.ManifestName]manifestNotifyCancel),
		fsWatcherMaker:  watcherMaker,
		timerMaker:      timerMaker,
	}
}

func (w *WatchManager) DisableForTesting() {
	w.disabledForTesting = true
}

func (w *WatchManager) diff(ctx context.Context, st store.RStore) (setup []WatchableManifest, teardown []WatchableManifest) {
	state := st.RLockState()
	defer st.RUnlockState()

	setup = []WatchableManifest{}
	teardown = []WatchableManifest{}
	manifestsToProcess := make(map[model.ManifestName]WatchableManifest, len(state.ManifestStates))
	for i, m := range state.ManifestStates {
		manifestsToProcess[i] = m.Manifest
	}
	if len(state.ConfigFiles) > 0 {
		manifestsToProcess["Tiltfile"] = &tiltfileManifest{dependencies: append([]string(nil), state.ConfigFiles...)}
	}
	for k, v := range manifestsToProcess {
		if _, ok := w.manifestWatches[k]; !ok {
			setup = append(setup, v)
		}
		delete(manifestsToProcess, k)
	}

	for _, v := range manifestsToProcess {
		teardown = append(teardown, v)
	}

	return setup, teardown
}

func (w *WatchManager) OnChange(ctx context.Context, st store.RStore) {
	setup, teardown := w.diff(ctx, st)

	for _, m := range teardown {
		p, ok := w.manifestWatches[m.ManifestName()]
		if !ok {
			continue
		}
		err := p.notify.Close()
		if err != nil {
			logger.Get(ctx).Infof("Error closing watch: %v", err)
		}
		p.cancel()
		delete(w.manifestWatches, m.ManifestName())
	}

	for _, manifest := range setup {
		watcher, err := w.fsWatcherMaker()
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			continue
		}

		for _, d := range manifest.Dependencies() {
			err = watcher.Add(d)
			if err != nil {
				st.Dispatch(NewErrorAction(err))
			}
		}

		ctx, cancel := context.WithCancel(ctx)

		go w.dispatchFileChangesLoop(ctx, manifest, watcher, st)

		w.manifestWatches[manifest.ManifestName()] = manifestNotifyCancel{manifest, watcher, cancel}
	}
}

func (w *WatchManager) dispatchFileChangesLoop(ctx context.Context, manifest WatchableManifest, watcher watch.Notify, st store.RStore) {
	filter, err := ignore.CreateFileChangeFilter(manifest)
	if err != nil {
		st.Dispatch(NewErrorAction(err))
		return
	}

	eventsCh := coalesceEvents(w.timerMaker, watcher.Events())

	for {
		select {
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			st.Dispatch(NewErrorAction(err))
		case <-ctx.Done():
			return

		case fsEvents, ok := <-eventsCh:
			if !ok {
				return
			}

			watchEvent := manifestFilesChangedAction{manifestName: manifest.ManifestName()}

			for _, e := range fsEvents {
				path, err := filepath.Abs(e.Path)
				if err != nil {
					st.Dispatch(NewErrorAction(err))
					continue
				}
				isIgnored, err := filter.Matches(path, false)
				if err != nil {
					st.Dispatch(NewErrorAction(err))
					continue
				}
				if !isIgnored {
					watchEvent.files = append(watchEvent.files, path)
				}
			}

			if len(watchEvent.files) > 0 {
				st.Dispatch(watchEvent)
			}
		}
	}
}

//makes an attempt to read some events from `eventChan` so that multiple file changes that happen at the same time
//from the user's perspective are grouped together.
func coalesceEvents(timerMaker timerMaker, eventChan <-chan watch.FileEvent) <-chan []watch.FileEvent {
	ret := make(chan []watch.FileEvent)
	go func() {
		defer close(ret)

		for {
			event, ok := <-eventChan
			if !ok {
				return
			}
			events := []watch.FileEvent{event}

			// keep grabbing changes until we've gone `watchBufferMinRestDuration` without seeing a change
			minRestTimer := timerMaker(watchBufferMinRestDuration)

			// but if we go too long before seeing a break (e.g., a process is constantly writing logs to that dir)
			// then just send what we've got
			timeout := timerMaker(watchBufferMaxDuration)

			done := false
			channelClosed := false
			for !done && !channelClosed {
				select {
				case event, ok := <-eventChan:
					if !ok {
						channelClosed = true
					} else {
						minRestTimer = timerMaker(watchBufferMinRestDuration)
						events = append(events, event)
					}
				case <-minRestTimer:
					done = true
				case <-timeout:
					done = true
				}
			}
			if len(events) > 0 {
				ret <- events
			}

			if channelClosed {
				return
			}
		}

	}()
	return ret
}
