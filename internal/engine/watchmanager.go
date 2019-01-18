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

var ConfigsTargetID = model.TargetID{
	Type: model.TargetTypeConfigs,
	Name: "singleton",
}

// If you modify this interface, you might also need to update the watchRulesMatch function below.
type WatchableTarget interface {
	ignore.IgnorableTarget
	Dependencies() []string
	ID() model.TargetID
}

// configTarget makes a WatchableTarget that works just for the config files (Tiltfile, yaml, Dockerfiles, etc.)
type configsTarget struct {
	dependencies []string
}

var _ WatchableTarget = &configsTarget{}

func (m *configsTarget) Dependencies() []string {
	return m.dependencies
}

func (m *configsTarget) ID() model.TargetID {
	return ConfigsTargetID
}

func (m *configsTarget) LocalRepos() []model.LocalGitRepo {
	return nil
}

func (m *configsTarget) Dockerignores() []model.Dockerignore {
	return nil
}

func (m *configsTarget) IgnoredLocalDirectories() []string {
	return nil
}

type targetFilesChangedAction struct {
	targetID model.TargetID
	files    []string
}

func (targetFilesChangedAction) Action() {}

type targetNotifyCancel struct {
	target WatchableTarget
	notify watch.Notify
	cancel func()
}

type WatchManager struct {
	targetWatches      map[model.TargetID]targetNotifyCancel
	fsWatcherMaker     FsWatcherMaker
	timerMaker         timerMaker
	disabledForTesting bool
}

func NewWatchManager(watcherMaker FsWatcherMaker, timerMaker timerMaker) *WatchManager {
	return &WatchManager{
		targetWatches:  make(map[model.TargetID]targetNotifyCancel),
		fsWatcherMaker: watcherMaker,
		timerMaker:     timerMaker,
	}
}

func (w *WatchManager) DisableForTesting() {
	w.disabledForTesting = true
}

func (w *WatchManager) diff(ctx context.Context, st store.RStore) (setup []WatchableTarget, teardown []model.TargetID) {
	state := st.RLockState()
	defer st.RUnlockState()

	setup = []WatchableTarget{}
	teardown = []model.TargetID{}

	targetsToProcess := make(map[model.TargetID]WatchableTarget)
	for _, m := range state.Manifests() {
		if m.IsDC() {
			dcTarget := m.DockerComposeTarget()
			targetsToProcess[dcTarget.ID()] = dcTarget
		}

		for _, iTarget := range m.ImageTargets {
			targetsToProcess[iTarget.ID()] = iTarget
		}
	}

	if len(state.ConfigFiles) > 0 {
		targetsToProcess[ConfigsTargetID] = &configsTarget{dependencies: append([]string(nil), state.ConfigFiles...)}
	}

	for name, mnc := range w.targetWatches {
		m, ok := targetsToProcess[name]
		if !ok {
			teardown = append(teardown, name)
			continue
		}

		if !watchRulesMatch(m, mnc.target) {
			teardown = append(teardown, name)
			setup = append(setup, m)
			break
		}
	}

	for name, m := range targetsToProcess {
		if _, ok := w.targetWatches[name]; !ok {
			setup = append(setup, m)
		}
		delete(targetsToProcess, name)
	}

	return setup, teardown
}

func watchRulesMatch(w1, w2 WatchableTarget) bool {
	if len(w1.LocalRepos()) != len(w2.LocalRepos()) {
		return false
	}
	for i, r := range w1.LocalRepos() {
		or := w2.LocalRepos()[i]
		if r.LocalPath != or.LocalPath || r.GitignoreContents != or.GitignoreContents {
			return false
		}
	}

	if len(w1.Dockerignores()) != len(w2.Dockerignores()) {
		return false
	}
	for i, di := range w1.Dockerignores() {
		odi := w2.Dockerignores()[i]
		if di.LocalPath != odi.LocalPath || di.Contents != odi.Contents {
			return false
		}
	}

	if len(w1.Dependencies()) != len(w2.Dependencies()) {
		return false
	}
	for i, d := range w1.Dependencies() {
		od := w2.Dependencies()[i]
		if d != od {
			return false
		}
	}

	if len(w1.IgnoredLocalDirectories()) != len(w2.IgnoredLocalDirectories()) {
		return false
	}
	for i, d := range w1.IgnoredLocalDirectories() {
		od := w2.IgnoredLocalDirectories()[i]
		if d != od {
			return false
		}
	}

	return true
}

func (w *WatchManager) OnChange(ctx context.Context, st store.RStore) {
	setup, teardown := w.diff(ctx, st)

	// setup the watch first, to avoid a gap in coverage between setup and
	// teardown. it's ok if we get a file event twice.
	newWatches := make(map[model.TargetID]targetNotifyCancel)
	for _, target := range setup {
		watcher, err := w.fsWatcherMaker()
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			continue
		}

		for _, d := range target.Dependencies() {
			err = watcher.Add(d)
			if err != nil {
				st.Dispatch(NewErrorAction(err))
			}
		}

		ctx, cancel := context.WithCancel(ctx)

		go w.dispatchFileChangesLoop(ctx, target, watcher, st)
		newWatches[target.ID()] = targetNotifyCancel{target, watcher, cancel}
	}

	for _, name := range teardown {
		p, ok := w.targetWatches[name]
		if !ok {
			continue
		}
		err := p.notify.Close()
		if err != nil {
			logger.Get(ctx).Infof("Error closing watch for %s: %v", name, err)
		}
		p.cancel()
		delete(w.targetWatches, name)
	}

	for k, v := range newWatches {
		w.targetWatches[k] = v
	}
}

func (w *WatchManager) dispatchFileChangesLoop(ctx context.Context, target WatchableTarget, watcher watch.Notify, st store.RStore) {
	filter, err := ignore.CreateFileChangeFilter(target)
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
			watchEvent := targetFilesChangedAction{targetID: target.ID()}

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
