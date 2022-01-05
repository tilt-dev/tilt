package filewatch

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// MaxFileEventHistory is the maximum number of file events that will be retained on the FileWatch status.
const MaxFileEventHistory = 20

const DetectedOverflowErrMsg = `It looks like the inotify event queue has overflowed. Check these instructions for how to raise the queue limit: https://facebook.github.io/watchman/docs/install#system-specific-preparation`

type watcher struct {
	clock          clockwork.Clock
	name           types.NamespacedName
	spec           v1alpha1.FileWatchSpec
	status         *v1alpha1.FileWatchStatus
	mu             sync.Mutex
	restartBackoff time.Duration
	doneAt         time.Time
	done           bool
	notify         watch.Notify
	cancel         func()
}

// Whether we need to restart the watcher.
func (w *watcher) shouldRestart() (bool, ctrl.Result) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.done {
		return false, ctrl.Result{}
	}

	if w.clock.Since(w.doneAt) < w.restartBackoff {
		return false, ctrl.Result{RequeueAfter: w.restartBackoff - w.clock.Since(w.doneAt)}
	}
	return true, ctrl.Result{}
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

	w.restartBackoff = w.restartBackoff * 2
	w.doneAt = w.clock.Now()
	if ctx.Err() == nil && w.status.Error == "" {
		w.status.Error = "unexpected close"
	}

	w.cancel()
	w.done = true
}

func (w *watcher) copyStatus() *v1alpha1.FileWatchStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status.DeepCopy()
}

func (w *watcher) recordError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		w.status.Error = ""
	} else {
		w.status.Error = err.Error()
	}
}

func (w *watcher) recordEvent(fsEvents []watch.FileEvent) {
	now := apis.NowMicro()
	w.mu.Lock()
	defer w.mu.Unlock()
	event := v1alpha1.FileEvent{Time: *now.DeepCopy()}
	for _, fsEvent := range fsEvents {
		event.SeenFiles = append(event.SeenFiles, fsEvent.Path())
	}
	if len(event.SeenFiles) != 0 {
		w.status.LastEventTime = *now.DeepCopy()
		w.status.FileEvents = append(w.status.FileEvents, event)
		if len(w.status.FileEvents) > MaxFileEventHistory {
			w.status.FileEvents = w.status.FileEvents[len(w.status.FileEvents)-MaxFileEventHistory:]
		}
		w.status.Error = ""
	}
}
