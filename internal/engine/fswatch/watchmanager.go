package fswatch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tilt-dev/fsnotify"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// When we see a file change, wait this long to see if any other files have changed, and bundle all changes together.
// 200ms is not the result of any kind of research or experimentation
// it might end up being a significant part of deployment delay, if we get the total latency <2s
// it might also be long enough that it misses some changes if the user has some operation involving a large file
//   (e.g., a binary dependency in git), but that's hopefully less of a problem since we'd get it in the next build
const BufferMinRestDuration = 200 * time.Millisecond

// When waiting for a `watchBufferDurationInMs`-long break in file modifications to aggregate notifications,
// if we haven't seen a break by the time `watchBufferMaxTimeInMs` has passed, just send off whatever we've got
const BufferMaxDuration = 10 * time.Second

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
var _ WatchableTarget = model.DockerComposeTarget{}

type targetNotifyCancel struct {
	name   types.NamespacedName
	spec   filewatches.FileWatchSpec
	status *filewatches.FileWatchStatus
	notify watch.Notify
	cancel func()
}

type WatchManager struct {
	targetWatches  map[types.NamespacedName]targetNotifyCancel
	fsWatcherMaker FsWatcherMaker
	timerMaker     TimerMaker
	mu             sync.Mutex
}

func NewWatchManager(watcherMaker FsWatcherMaker, timerMaker TimerMaker) *WatchManager {
	return &WatchManager{
		targetWatches:  make(map[types.NamespacedName]targetNotifyCancel),
		fsWatcherMaker: watcherMaker,
		timerMaker:     timerMaker,
	}
}

func (w *WatchManager) TargetWatchCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.targetWatches)
}

func (w *WatchManager) diff(st store.RStore, summary store.ChangeSummary) map[types.NamespacedName]*filewatches.FileWatch {
	state := st.RLockState()
	defer st.RUnlockState()

	if !state.EngineMode.WatchesFiles() {
		return nil
	}

	result := make(map[types.NamespacedName]*filewatches.FileWatch)
	for key := range summary.FileWatchSpecs {
		result[key] = state.FileWatches[key]
	}
	return result
}

func (w *WatchManager) reconcile(ctx context.Context, st store.RStore, name types.NamespacedName, fw *filewatches.FileWatch) {
	existing, hasExisting := w.targetWatches[name]

	if fw == nil || fw.GetObjectMeta().GetDeletionTimestamp() != nil {
		if hasExisting {
			w.cleanupWatch(ctx, existing)
			delete(w.targetWatches, name)
		}
		return
	}

	if hasExisting && equality.Semantic.DeepEqual(existing.spec, fw.Spec) {
		return
	}

	if err := w.addOrReplace(ctx, st, name, *fw.Spec.DeepCopy()); err != nil {
		logger.Get(ctx).Debugf("Failed to create/update filesystem watch for FileWatch %q: %v", name.String(), err)
	}
}

func (w *WatchManager) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	w.mu.Lock()
	defer w.mu.Unlock()

	toReconcile := w.diff(st, summary)
	for name, fw := range toReconcile {
		w.reconcile(ctx, st, name, fw)
	}
}

func (w *WatchManager) addOrReplace(ctx context.Context, st store.RStore, name types.NamespacedName, spec filewatches.FileWatchSpec) error {
	var ignoreMatchers []model.PathMatcher
	for _, ignoreDef := range spec.Ignores {
		if len(ignoreDef.Patterns) != 0 {
			m, err := dockerignore.NewDockerPatternMatcher(
				ignoreDef.BasePath,
				append([]string{}, ignoreDef.Patterns...))
			if err != nil {
				return fmt.Errorf("invalid ignore def: %v", err)
			}
			ignoreMatchers = append(ignoreMatchers, m)
		} else {
			m, err := ignore.NewDirectoryMatcher(ignoreDef.BasePath)
			if err != nil {
				return fmt.Errorf("invalid ignore def: %v", err)
			}
			ignoreMatchers = append(ignoreMatchers, m)
		}
	}
	// ephemeral OS/IDE stuff is not part of the spec but always included
	ignoreMatchers = append(ignoreMatchers, ignore.EphemeralPathMatcher)

	notify, err := w.fsWatcherMaker(
		append([]string{}, spec.WatchedPaths...),
		model.NewCompositeMatcher(ignoreMatchers),
		logger.Get(ctx))
	if err != nil {
		return fmt.Errorf("failed to initialize filesystem watch: %v", err)
	}
	if err := notify.Start(); err != nil {
		return fmt.Errorf("failed to initialize filesystem watch: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	tw := targetNotifyCancel{
		name:   name,
		spec:   *spec.DeepCopy(),
		status: &filewatches.FileWatchStatus{},
		notify: notify,
		cancel: cancel,
	}

	go w.dispatchFileChangesLoop(ctx, name, notify, st)

	if existing, ok := w.targetWatches[name]; ok {
		// no need to remove from map, will be overwritten
		w.cleanupWatch(ctx, existing)
	}

	w.targetWatches[name] = tw
	return nil
}

// cleanupWatch stops watching for changes and frees up resources.
//
// It does NOT remove the entry from the map - to avoid missed updates, entries are typically just overwritten.
func (w *WatchManager) cleanupWatch(ctx context.Context, tw targetNotifyCancel) {
	if err := tw.notify.Close(); err != nil {
		logger.Get(ctx).Debugf("Failed to close notifier for %q: %v", tw.name.String(), err)
	}
	tw.cancel()
}

func (w *WatchManager) dispatchFileChangesLoop(
	ctx context.Context,
	name types.NamespacedName,
	watcher watch.Notify,
	st store.RStore) {

	eventsCh := coalesceEvents(w.timerMaker, watcher.Events())

	for {
		select {
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}

			// TODO(milas): these should probably update the error field and emit FileWatchUpdateAction
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

			now := metav1.NowMicro()
			w.mu.Lock()
			event := filewatches.FileEvent{Time: *now.DeepCopy()}
			for _, fsEvent := range fsEvents {
				event.SeenFiles = append(event.SeenFiles, fsEvent.Path())
			}
			if len(event.SeenFiles) != 0 {
				status := w.targetWatches[name].status
				status.LastEventTime = now.DeepCopy()
				// TODO(milas): cap max event history
				status.FileEvents = append(status.FileEvents, event)
				st.Dispatch(NewFileWatchUpdateStatusAction(name, status))
			}
			w.mu.Unlock()
		}
	}
}

// coalesceEvents makes an attempt to read some events from `eventChan` so that multiple file changes
// that happen at the same time from the user's perspective are grouped together.
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
