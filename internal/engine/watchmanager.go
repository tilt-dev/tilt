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

// TODO(maia): throw an error if you try to name a manifest this in your Tiltfile?
const ConfigsManifestName = "_ConfigsManifest"

type WatchableManifest interface {
	Dependencies() []string
	ManifestName() model.ManifestName
	LocalRepos() []model.LocalGitRepo
	Dockerignores() []model.Dockerignore
	// These directories and their children will not trigger file change events
	IgnoredLocalDirectories() []string
}

type dcManifest struct {
	name model.ManifestName
	model.DockerComposeTarget
}

func (m dcManifest) ManifestName() model.ManifestName {
	return m.name
}

type imageManifest struct {
	name model.ManifestName
	model.ImageTarget
}

func (m imageManifest) ManifestName() model.ManifestName {
	return m.name
}

// configManifest makes a WatchableManifest that works just for the config files (Tiltfile, yaml, Dockerfiles, etc.)
type configsManifest struct {
	dependencies []string
}

var _ WatchableManifest = &configsManifest{}

func (m *configsManifest) Dependencies() []string {
	return m.dependencies
}

func (m *configsManifest) ManifestName() model.ManifestName {
	return ConfigsManifestName
}

func (m *configsManifest) LocalRepos() []model.LocalGitRepo {
	return nil
}

func (m *configsManifest) Dockerignores() []model.Dockerignore {
	return nil
}

func (m *configsManifest) IgnoredLocalDirectories() []string {
	return nil
}

type manifestFilesChangedAction struct {
	manifestName model.ManifestName
	files        []string
}

func (manifestFilesChangedAction) Action() {}

type manifestNotifyCancel struct {
	manifest WatchableManifest
	notify   watch.Notify
	cancel   func()
}

type WatchManager struct {
	manifestWatches    map[model.ManifestName]manifestNotifyCancel
	fsWatcherMaker     FsWatcherMaker
	timerMaker         timerMaker
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

func (w *WatchManager) diff(ctx context.Context, st store.RStore) (setup []WatchableManifest, teardown []model.ManifestName) {
	state := st.RLockState()
	defer st.RUnlockState()

	setup = []WatchableManifest{}
	teardown = []model.ManifestName{}

	manifestsToProcess := make(map[model.ManifestName]WatchableManifest)
	for _, m := range state.Manifests() {
		if m.IsDC() {
			manifestsToProcess[m.Name] = dcManifest{name: m.Name, DockerComposeTarget: m.DockerComposeTarget()}
		} else {
			manifestsToProcess[m.Name] = imageManifest{name: m.Name, ImageTarget: m.ImageTarget}
		}
	}

	if len(state.ConfigFiles) > 0 {
		manifestsToProcess[ConfigsManifestName] = &configsManifest{dependencies: append([]string(nil), state.ConfigFiles...)}
	}

	for name, mnc := range w.manifestWatches {
		m, ok := manifestsToProcess[name]
		if !ok {
			teardown = append(teardown, name)
			continue
		}

		if !dependenciesMatch(m.Dependencies(), mnc.manifest.Dependencies()) {
			teardown = append(teardown, name)
			setup = append(setup, m)
			break
		}
	}

	for name, m := range manifestsToProcess {
		if _, ok := w.manifestWatches[name]; !ok {
			setup = append(setup, m)
		}
		delete(manifestsToProcess, name)
	}

	return setup, teardown
}

func dependenciesMatch(d1 []string, d2 []string) bool {
	if len(d1) != len(d2) {
		return false
	}

	for i, e1 := range d1 {
		e2 := d2[i]
		if e1 != e2 {
			return false
		}
	}

	return true
}

func (w *WatchManager) OnChange(ctx context.Context, st store.RStore) {
	setup, teardown := w.diff(ctx, st)

	// setup the watch first, to avoid a gap in coverage between setup and
	// teardown. it's ok if we get a file event twice.
	newWatches := make(map[model.ManifestName]manifestNotifyCancel)
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
		newWatches[manifest.ManifestName()] = manifestNotifyCancel{manifest, watcher, cancel}
	}

	for _, name := range teardown {
		p, ok := w.manifestWatches[name]
		if !ok {
			continue
		}
		err := p.notify.Close()
		if err != nil {
			logger.Get(ctx).Infof("Error closing watch for %s: %v", name, err)
		}
		p.cancel()
		delete(w.manifestWatches, name)
	}

	for k, v := range newWatches {
		w.manifestWatches[k] = v
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
