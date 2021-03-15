package filewatch

import (
	"context"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/engine/fswatch"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// MaxFileEventHistory is the maximum number of file events that will be retained on the FileWatch status.
const MaxFileEventHistory = 20

const DetectedOverflowErrMsg = `It looks like the inotify event queue has overflowed. Check these instructions for how to raise the queue limit: https://facebook.github.io/watchman/docs/install#system-specific-preparation`

type watcher struct {
	name   types.NamespacedName
	spec   filewatches.FileWatchSpec
	status *filewatches.FileWatchStatus
	mu     sync.Mutex
	done   bool
	notify watch.Notify
	cancel func()
}

// cleanupWatch stops watching for changes and frees up resources.
func (w *watcher) cleanupWatch(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done {
		return
	}
	if err := w.notify.Close(); err != nil {
		logger.Get(ctx).Debugf("Failed to close notifier for %q: %v", w.name.String(), err)
	}
	w.cancel()
	w.done = true
}

func (w *watcher) recordEvent(ctx context.Context, client ctrlclient.Client, st store.RStore, fsEvents []watch.FileEvent) error {
	now := metav1.NowMicro()
	w.mu.Lock()
	defer w.mu.Unlock()
	event := filewatches.FileEvent{Time: *now.DeepCopy()}
	for _, fsEvent := range fsEvents {
		event.SeenFiles = append(event.SeenFiles, fsEvent.Path())
	}
	if len(event.SeenFiles) != 0 {
		logger.Get(ctx).Debugf("File event for %q: %v", w.name.String(), event.SeenFiles)
		w.status.LastEventTime = *now.DeepCopy()
		w.status.FileEvents = append(w.status.FileEvents, event)
		if len(w.status.FileEvents) > MaxFileEventHistory {
			w.status.FileEvents = w.status.FileEvents[len(w.status.FileEvents)-MaxFileEventHistory:]
		}

		var fw filewatches.FileWatch
		err := client.Get(ctx, w.name, &fw)
		if err != nil {
			// status is updated internally so will become eventually consistent, but if there's no file
			// changes for a while after this, the updates aren't going to appear; retry logic is probably
			// warranted here
			return nil
		}

		w.status.DeepCopyInto(&fw.Status)
		err = client.Status().Update(ctx, &fw)
		if err == nil {
			st.Dispatch(fswatch.NewFileWatchUpdateStatusAction(&fw))
		} else if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			// can safely ignore not found/conflict errors - because this work loop is the only updater of
			// status, any conflict errors means the spec was changed since fetching it, and as a result,
			// these events are no longer useful anyway
			return fmt.Errorf("apiserver update status error: %v", err)
		}
	}
	return nil
}
