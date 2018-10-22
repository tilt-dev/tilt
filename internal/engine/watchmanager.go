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

type manifestFilesChangedAction struct {
	manifestName model.ManifestName
	files        []string
}

func (manifestFilesChangedAction) Action() {}

type manifestNotifyCancel struct {
	manifest model.Manifest
	notify   watch.Notify
	cancel   func()
}

type WatchManager struct {
	watches        map[model.ManifestName]manifestNotifyCancel
	fsWatcherMaker FsWatcherMaker
	timerMaker     timerMaker
}

func NewWatchManager(watcherMaker FsWatcherMaker, timerMaker timerMaker) *WatchManager {
	return &WatchManager{
		watches:        make(map[model.ManifestName]manifestNotifyCancel),
		fsWatcherMaker: watcherMaker,
		timerMaker:     timerMaker,
	}
}

func (w *WatchManager) diff(ctx context.Context, st *store.Store) (setup []model.Manifest, teardown []model.Manifest) {
	state := st.RLockState()
	defer st.RUnlockState()

	setup = []model.Manifest{}
	teardown = []model.Manifest{}
	manifestsToProcess := make(map[model.ManifestName]model.Manifest, len(state.ManifestStates))
	for i, m := range state.ManifestStates {
		manifestsToProcess[i] = m.Manifest
	}
	for n, state := range state.ManifestStates {
		_, ok := w.watches[n]
		if !ok {
			setup = append(setup, state.Manifest)
		}
		delete(manifestsToProcess, n)
	}

	for _, m := range manifestsToProcess {
		teardown = append(teardown, m)
	}

	return setup, teardown
}

func (w *WatchManager) OnChange(ctx context.Context, st *store.Store) {
	setup, teardown := w.diff(ctx, st)

	for _, m := range teardown {
		p, ok := w.watches[m.Name]
		if !ok {
			continue
		}
		err := p.notify.Close()
		if err != nil {
			logger.Get(ctx).Infof("Error closing watch: %v", err)
		}
		p.cancel()
		delete(w.watches, m.Name)
	}

	for _, manifest := range setup {
		watcher, err := w.fsWatcherMaker()
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			continue
		}

		localPaths := manifest.LocalPaths()

		for _, localPath := range localPaths {
			err = watcher.Add(localPath)
			if err != nil {
				st.Dispatch(NewErrorAction(err))
			}
		}

		for _, cf := range manifest.ConfigFiles {
			err = watcher.Add(cf)
			if err != nil {
				st.Dispatch(NewErrorAction(err))
			}
		}

		ctx, cancel := context.WithCancel(ctx)

		go w.dispatchFileChangesLoop(ctx, manifest, watcher, st)

		w.watches[manifest.Name] = manifestNotifyCancel{manifest, watcher, cancel}
	}
}

func (w *WatchManager) dispatchFileChangesLoop(ctx context.Context, manifest model.Manifest, watcher watch.Notify, st *store.Store) {
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

			watchEvent := manifestFilesChangedAction{manifestName: manifest.Name}

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
