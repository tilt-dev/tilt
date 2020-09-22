package fswatch

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tilt-dev/fsnotify"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// When we see a file change, wait this long to see if any other files have changed, and bundle all changes together.
// 200ms is not the result of any kind of research or experimentation
// it might end up being a significant part of deployment delay, if we get the total latency <2s
// it might also be long enough that it misses some changes if the user has some operation involving a large file
//   (e.g., a binary dependency in git), but that's hopefully less of a problem since we'd get it in the next build
const BufferMinRestInMs = 200

// When waiting for a `watchBufferDurationInMs`-long break in file modifications to aggregate notifications,
// if we haven't seen a break by the time `watchBufferMaxTimeInMs` has passed, just send off whatever we've got
const BufferMaxTimeInMs = 10000

var BufferMinRestDuration = BufferMinRestInMs * time.Millisecond
var BufferMaxDuration = BufferMaxTimeInMs * time.Millisecond

const DetectedOverflowErrMsg = `It looks like the inotify event queue has overflowed. Check these instructions for how to raise the queue limit: https://facebook.github.io/watchman/docs/install#system-specific-preparation`

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

var _ WatchableTarget = model.ImageTarget{}
var _ WatchableTarget = model.LocalTarget{}

func WatchableTargetsForManifests(manifests []model.Manifest) []WatchableTarget {
	var watchable []WatchableTarget
	seen := map[model.TargetID]bool{}
	for _, m := range manifests {
		for _, t := range m.TargetSpecs() {
			if !seen[t.ID()] {
				if watchTarg, ok := t.(WatchableTarget); ok {
					watchable = append(watchable, watchTarg)
					seen[watchTarg.ID()] = true
				}
			}
		}
	}
	return watchable
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

type targetNotifyCancel struct {
	target WatchableTarget
	notify watch.Notify
	cancel func()
}

type WatchManager struct {
	targetWatches      map[model.TargetID]targetNotifyCancel
	fsWatcherMaker     FsWatcherMaker
	timerMaker         TimerMaker
	globalIgnores      []model.Dockerignore
	globalIgnore       model.PathMatcher
	disabledForTesting bool
	mu                 sync.Mutex
}

func NewWatchManager(watcherMaker FsWatcherMaker, timerMaker TimerMaker) *WatchManager {
	return &WatchManager{
		targetWatches:  make(map[model.TargetID]targetNotifyCancel),
		fsWatcherMaker: watcherMaker,
		timerMaker:     timerMaker,
		globalIgnore:   model.EmptyMatcher,
	}
}

func (w *WatchManager) DisableForTesting() {
	w.disabledForTesting = true
}

func (w *WatchManager) diff(ctx context.Context, st store.RStore) (setup []WatchableTarget, teardown []model.TargetID) {
	state := st.RLockState()
	defer st.RUnlockState()

	if !state.EngineMode.WatchesFiles() {
		return nil, nil
	}

	setup = []WatchableTarget{}
	teardown = []model.TargetID{}

	watchable := WatchableTargetsForManifests(state.Manifests())
	targetsToProcess := make(map[model.TargetID]WatchableTarget)
	for _, w := range watchable {
		targetsToProcess[w.ID()] = w
	}

	if len(state.ConfigFiles) > 0 {
		targetsToProcess[ConfigsTargetID] = &configsTarget{dependencies: append([]string(nil), state.ConfigFiles...)}
	}

	newGlobalIgnores := globalIgnores(state)
	globalIgnoreChanged := !cmp.Equal(newGlobalIgnores, w.globalIgnores, cmpopts.EquateEmpty())

	for name, mnc := range w.targetWatches {
		m, ok := targetsToProcess[name]
		if !ok {
			teardown = append(teardown, name)
			continue
		}

		if globalIgnoreChanged || !watchRulesMatch(m, mnc.target) {
			teardown = append(teardown, name)
			setup = append(setup, m)
		}
	}

	for name, m := range targetsToProcess {
		if _, ok := w.targetWatches[name]; !ok {
			setup = append(setup, m)
		}
		delete(targetsToProcess, name)
	}

	if globalIgnoreChanged {
		w.globalIgnores = newGlobalIgnores

		globalIgnoreFilter, err := dockerignoresToMatcher(newGlobalIgnores)
		if err != nil {
			st.Dispatch(store.NewErrorAction(err))
		}
		w.globalIgnore = globalIgnoreFilter
	}

	return setup, teardown
}

// Return a list of global ignore patterns.
func globalIgnores(es store.EngineState) []model.Dockerignore {
	ignores := []model.Dockerignore{}
	if !es.Tiltignore.Empty() {
		ignores = append(ignores, es.Tiltignore)
	}
	ignores = append(ignores, es.WatchSettings.Ignores...)

	outputs := []string{}
	for _, manifest := range es.Manifests() {
		for _, iTarget := range manifest.ImageTargets {
			customBuild := iTarget.CustomBuildInfo()
			if customBuild.OutputsImageRefTo != "" {
				outputs = append(outputs, customBuild.OutputsImageRefTo)
			}
		}
	}

	if len(outputs) > 0 {
		ignores = append(ignores, model.Dockerignore{
			LocalPath: filepath.Dir(es.TiltfilePath),
			Source:    "outputs_image_ref_to",
			Patterns:  outputs,
		})
	}

	return ignores
}

func dockerignoresToMatcher(ignores []model.Dockerignore) (model.PathMatcher, error) {
	matchers := make([]model.PathMatcher, 0, len(ignores))
	for _, ignore := range ignores {
		matcher, err := dockerignore.NewDockerPatternMatcher(ignore.LocalPath, ignore.Patterns)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, matcher)
	}
	return model.NewCompositeMatcher(matchers), nil
}

func watchRulesMatch(w1, w2 WatchableTarget) bool {
	return cmp.Equal(w1.LocalRepos(), w2.LocalRepos()) &&
		cmp.Equal(w1.Dockerignores(), w2.Dockerignores()) &&
		cmp.Equal(w1.Dependencies(), w2.Dependencies()) &&
		cmp.Equal(w1.IgnoredLocalDirectories(), w2.IgnoredLocalDirectories())
}

func (w *WatchManager) TargetWatchCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.targetWatches)
}

func (w *WatchManager) OnChange(ctx context.Context, st store.RStore) {
	w.mu.Lock()
	defer w.mu.Unlock()

	setup, teardown := w.diff(ctx, st)

	// setup the watch first, to avoid a gap in coverage between setup and
	// teardown. it's ok if we get a file event twice.
	newWatches := make(map[model.TargetID]targetNotifyCancel)
	for _, target := range setup {
		logger := store.NewLogActionLogger(ctx, st.Dispatch)
		ignore, err := w.createIgnoreMatcher(target)
		if err != nil {
			st.Dispatch(store.NewErrorAction(err))
			continue
		}

		watcher, err := w.fsWatcherMaker(target.Dependencies(), ignore, logger)
		if err != nil {
			st.Dispatch(store.NewErrorAction(err))
			continue
		}

		err = watcher.Start()
		if err != nil {
			st.Dispatch(store.NewErrorAction(err))
			continue
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

func (w *WatchManager) createIgnoreMatcher(target WatchableTarget) (watch.PathMatcher, error) {
	filter, err := ignore.CreateFileChangeFilter(target)
	if err != nil {
		return nil, err
	}
	return model.NewCompositeMatcher([]model.PathMatcher{filter, w.globalIgnore}), nil
}

func (w *WatchManager) dispatchFileChangesLoop(
	ctx context.Context,
	target WatchableTarget,
	watcher watch.Notify,
	st store.RStore) {

	eventsCh := coalesceEvents(w.timerMaker, watcher.Events())

	for {
		select {
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			if watch.IsWindowsShortReadError(err) {
				st.Dispatch(store.NewErrorAction(fmt.Errorf("Windows I/O overflow.\n"+
					"You may be able to fix this by setting the env var %s.\n"+
					"Current buffer size: %d\n"+
					"More details: https://github.com/tilt-dev/tilt/issues/3556\n"+
					"Caused by: %v",
					watch.WindowsBufferSizeEnvVar,
					watch.DesiredWindowsBufferSize(),
					err)))
			} else if err.Error() == fsnotify.ErrEventOverflow.Error() {
				st.Dispatch(store.NewErrorAction(fmt.Errorf("%s\nerror: %v", DetectedOverflowErrMsg, err)))
			} else {
				st.Dispatch(store.NewErrorAction(err))
			}
		case <-ctx.Done():
			return

		case fsEvents, ok := <-eventsCh:

			if !ok {
				return
			}
			watchEvent := NewTargetFilesChangedAction(target.ID())
			for _, e := range fsEvents {
				watchEvent.Files = append(watchEvent.Files, e.Path())
			}

			if len(watchEvent.Files) > 0 {
				st.Dispatch(watchEvent)
			}
		}
	}
}

//makes an attempt to read some events from `eventChan` so that multiple file changes that happen at the same time
//from the user's perspective are grouped together.
func coalesceEvents(timerMaker TimerMaker, eventChan <-chan watch.FileEvent) <-chan []watch.FileEvent {
	ret := make(chan []watch.FileEvent)
	go func() {
		defer close(ret)

		for {
			event, ok := <-eventChan
			if !ok {
				return
			}
			events := []watch.FileEvent{event}

			// keep grabbing changes until we've gone `BufferMinRestDuration` without seeing a change
			minRestTimer := timerMaker(BufferMinRestDuration)

			// but if we go too long before seeing a break (e.g., a process is constantly writing logs to that dir)
			// then just send what we've got
			timeout := timerMaker(BufferMaxDuration)

			done := false
			channelClosed := false
			for !done && !channelClosed {
				select {
				case event, ok := <-eventChan:
					if !ok {
						channelClosed = true
					} else {
						minRestTimer = timerMaker(BufferMinRestDuration)
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
