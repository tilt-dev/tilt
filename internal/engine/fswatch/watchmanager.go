package fswatch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tilt-dev/fsnotify"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

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

// MaxFileEventHistory is the maximum number of file events that will be retained on the FileWatch status.
const MaxFileEventHistory = 20

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
	name      types.NamespacedName
	spec      filewatches.FileWatchSpec
	updateObj *filewatches.FileWatch
	mu        sync.Mutex
	done      bool
	notify    watch.Notify
	cancel    func()
}

type WatchManager struct {
	targetWatches  map[types.NamespacedName]*targetNotifyCancel
	fsWatcherMaker FsWatcherMaker
	timerMaker     TimerMaker
	client         ctrlclient.Client
	mu             sync.Mutex
}

func NewWatchManager(watcherMaker FsWatcherMaker, timerMaker TimerMaker, client ctrlclient.Client) *WatchManager {
	return &WatchManager{
		targetWatches:  make(map[types.NamespacedName]*targetNotifyCancel),
		fsWatcherMaker: watcherMaker,
		timerMaker:     timerMaker,
		client:         client,
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
		if fw, ok := state.FileWatches[key]; ok {
			result[key] = fw.DeepCopy()
		} else {
			result[key] = nil
		}
	}
	return result
}

func (w *WatchManager) reconcile(ctx context.Context, st store.RStore, name types.NamespacedName, fw *filewatches.FileWatch) {
	existing, hasExisting := w.targetWatches[name]

	if fw == nil || fw.GetObjectMeta().GetDeletionTimestamp() != nil {
		if hasExisting {
			existing.cleanupWatch(ctx)
			delete(w.targetWatches, name)
		}
		return
	}

	if hasExisting && equality.Semantic.DeepEqual(existing.spec, fw.Spec) {
		return
	}

	if err := w.addOrReplace(ctx, st, name, *fw); err != nil {
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

func (w *WatchManager) addOrReplace(ctx context.Context, st store.RStore, name types.NamespacedName, fw filewatches.FileWatch) error {
	var ignoreMatchers []model.PathMatcher
	for _, ignoreDef := range fw.Spec.Ignores {
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
		append([]string{}, fw.Spec.WatchedPaths...),
		model.NewCompositeMatcher(ignoreMatchers),
		logger.Get(ctx))
	if err != nil {
		return fmt.Errorf("failed to initialize filesystem watch: %v", err)
	}
	if err := notify.Start(); err != nil {
		return fmt.Errorf("failed to initialize filesystem watch: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	tw := &targetNotifyCancel{
		name:      name,
		spec:      *fw.Spec.DeepCopy(),
		updateObj: fw.DeepCopy(),
		notify:    notify,
		cancel:    cancel,
	}

	go w.dispatchFileChangesLoop(ctx, st, tw)

	if existing, ok := w.targetWatches[name]; ok {
		// no need to remove from map, will be overwritten
		existing.cleanupWatch(ctx)
	}

	w.targetWatches[name] = tw
	return nil
}

// cleanupWatch stops watching for changes and frees up resources.
func (tw *targetNotifyCancel) cleanupWatch(ctx context.Context) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.done {
		return
	}
	if err := tw.notify.Close(); err != nil {
		logger.Get(ctx).Debugf("Failed to close notifier for %q: %v", tw.name.String(), err)
	}
	tw.cancel()
	tw.done = true
}

func (tw *targetNotifyCancel) recordEvent(ctx context.Context, client ctrlclient.Client, st store.RStore, fsEvents []watch.FileEvent) error {
	now := metav1.NowMicro()
	tw.mu.Lock()
	defer tw.mu.Unlock()
	event := filewatches.FileEvent{Time: *now.DeepCopy()}
	for _, fsEvent := range fsEvents {
		event.SeenFiles = append(event.SeenFiles, fsEvent.Path())
	}
	if len(event.SeenFiles) != 0 {
		tw.updateObj.Status.LastEventTime = now.DeepCopy()
		tw.updateObj.Status.FileEvents = append(tw.updateObj.Status.FileEvents, event)
		if len(tw.updateObj.Status.FileEvents) > MaxFileEventHistory {
			tw.updateObj.Status.FileEvents = tw.updateObj.Status.FileEvents[len(tw.updateObj.Status.FileEvents)-MaxFileEventHistory:]
		}

		err := client.Status().Update(ctx, tw.updateObj)
		if err == nil {
			st.Dispatch(NewFileWatchUpdateStatusAction(tw.updateObj))
		} else if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			// can safely ignore not found/conflict errors - because this work loop is the only updater of
			// status, any conflict errors means the spec was changed since fetching it, and as a result,
			// these events are no longer useful anyway
			return fmt.Errorf("apiserver update status error: %v", err)
		}
	}
	return nil
}

// removeWatch removes a watch from the map. It does NOT stop the watcher or free up resources.
//
// mu must be held before calling.
func (w *WatchManager) removeWatch(tw *targetNotifyCancel) {
	if entry, ok := w.targetWatches[tw.name]; ok && tw == entry {
		delete(w.targetWatches, tw.name)
	}
}

func (w *WatchManager) dispatchFileChangesLoop(ctx context.Context, st store.RStore, tw *targetNotifyCancel) {
	eventsCh := coalesceEvents(w.timerMaker, tw.notify.Events())

	defer func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		tw.cleanupWatch(ctx)
		w.removeWatch(tw)
	}()

	for {
		select {
		case err, ok := <-tw.notify.Errors():
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
			if err := tw.recordEvent(ctx, w.client, st, fsEvents); err != nil {
				st.Dispatch(store.NewErrorAction(err))
				return
			}
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
