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
	// TODO(dmiller) do I need a mutex here?
}

func NewWatchManager(watcherMaker FsWatcherMaker) *WatchManager {
	return &WatchManager{
		watches:        make(map[model.ManifestName]manifestNotifyCancel),
		fsWatcherMaker: watcherMaker,
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

		go dispatchFileChangesLoop(ctx, manifest, watcher, st)

		w.watches[manifest.Name] = manifestNotifyCancel{manifest, watcher, cancel}
	}
}

func dispatchFileChangesLoop(ctx context.Context, manifest model.Manifest, watcher watch.Notify, st *store.Store) {
	filter, err := ignore.CreateFileChangeFilter(manifest)
	if err != nil {
		st.Dispatch(NewErrorAction(err))
		return
	}

	for {
		select {
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			st.Dispatch(NewErrorAction(err))
		case <-ctx.Done():
			return

		case fsEvent, ok := <-watcher.Events():
			if !ok {
				return
			}

			watchEvent := manifestFilesChangedAction{manifestName: manifest.Name}

			path, err := filepath.Abs(fsEvent.Path)
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

			if len(watchEvent.files) > 0 {
				st.Dispatch(watchEvent)
			}
		}
	}
}
