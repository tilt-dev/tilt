package filewatch

import (
	"context"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
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
		w.status.LastEventTime = *now.DeepCopy()
		w.status.FileEvents = append(w.status.FileEvents, event)
		if len(w.status.FileEvents) > MaxFileEventHistory {
			w.status.FileEvents = w.status.FileEvents[len(w.status.FileEvents)-MaxFileEventHistory:]
		}

		// TODO(milas): we should only update the internal status and then trigger
		// 	reconciliation and let that handle updating status
		//
		// historically, this method did a PUT/replace, which meant file events
		// could be indefinitely delayed on optimistic concurrency failures due
		// to lack of retry
		//
		// now, we PATCH assuming the spec matches at the time of fetch, but this
		// means there is technically still a race if the spec changes between
		// our fetch and server processing the update, where we'll post "stale"
		// events, but this is less harmful than effectively missing files events
		var fw filewatches.FileWatch
		err := client.Get(ctx, w.name, &fw)
		if err != nil {
			// status is updated internally so will become eventually consistent,
			// but the file event won't be seen until the _next_ event!
			//
			// see the note above on how we should really resolve this
			return nil
		}
		if !apicmp.DeepEqual(w.spec, fw.Spec) {
			// spec changed - these events are outdated, so don't write them to the status
			return nil
		}

		// only modify the specific fields on status that we care about so we can do a PATCH
		patchBase := ctrlclient.MergeFrom(fw.DeepCopy())
		updateStatus := w.status.DeepCopy()
		fw.Status.LastEventTime = updateStatus.LastEventTime
		fw.Status.FileEvents = updateStatus.FileEvents

		err = client.Status().Patch(ctx, &fw, patchBase)
		if err == nil {
			st.Dispatch(NewFileWatchUpdateStatusAction(&fw))
		} else if !apierrors.IsNotFound(err) {
			// can safely ignore not found/conflict errors - because this work loop is the only updater of
			// status, any conflict errors means the spec was changed since fetching it, and as a result,
			// these events are no longer useful anyway
			return fmt.Errorf("apiserver update status error: %v", err)
		}
	}
	return nil
}
