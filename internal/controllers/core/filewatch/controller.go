/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filewatch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/tilt-dev/fsnotify"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch/fsevent"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/filewatches"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Controller reconciles a FileWatch object
type Controller struct {
	ctrlclient.Client
	Store store.RStore

	targetWatches  map[types.NamespacedName]*watcher
	fsWatcherMaker fsevent.WatcherMaker
	timerMaker     fsevent.TimerMaker
	mu             sync.Mutex
	clock          clockwork.Clock
	indexer        *indexer.Indexer
	requeuer       *indexer.Requeuer
}

func NewController(client ctrlclient.Client, store store.RStore, fsWatcherMaker fsevent.WatcherMaker, timerMaker fsevent.TimerMaker, scheme *runtime.Scheme, clock clockwork.Clock) *Controller {
	return &Controller{
		Client:         client,
		Store:          store,
		targetWatches:  make(map[types.NamespacedName]*watcher),
		fsWatcherMaker: fsWatcherMaker,
		timerMaker:     timerMaker,
		indexer:        indexer.NewIndexer(scheme, indexFw),
		requeuer:       indexer.NewRequeuer(),
		clock:          clock,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	existing, hasExisting := c.targetWatches[req.NamespacedName]

	var fw v1alpha1.FileWatch
	err := c.Client.Get(ctx, req.NamespacedName, &fw)

	c.indexer.OnReconcile(req.NamespacedName, &fw)

	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !fw.ObjectMeta.DeletionTimestamp.IsZero() {
		if hasExisting {
			existing.cleanupWatch(ctx)
			c.removeWatch(existing)
		}
		c.Store.Dispatch(filewatches.NewFileWatchDeleteAction(req.NamespacedName.Name))
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	c.Store.Dispatch(filewatches.NewFileWatchUpsertAction(&fw))

	ctx = store.MustObjectLogHandler(ctx, c.Store, &fw)

	// Get configmap's disable status
	disableStatus, err := configmap.MaybeNewDisableStatus(ctx, c.Client, fw.Spec.DisableSource, fw.Status.DisableStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Clean up existing filewatches if it's disabled
	result := ctrl.Result{}
	if disableStatus.State == v1alpha1.DisableStateDisabled {
		if hasExisting {
			existing.cleanupWatch(ctx)
			c.removeWatch(existing)
		}
	} else {
		// Determine if we the filewatch needs to be refreshed.
		shouldRestart := !hasExisting || !apicmp.DeepEqual(existing.spec, fw.Spec)
		if hasExisting && !shouldRestart {
			shouldRestart, result = existing.shouldRestart()
		}

		if shouldRestart {
			c.addOrReplace(ctx, req.NamespacedName, &fw)
		}
	}

	watch, ok := c.targetWatches[req.NamespacedName]
	status := &v1alpha1.FileWatchStatus{DisableStatus: disableStatus}
	if ok {
		status = watch.copyStatus()
		status.DisableStatus = disableStatus
	}

	err = c.maybeUpdateObjectStatus(ctx, &fw, status)
	if err != nil {
		return ctrl.Result{}, err
	}

	return result, nil
}

func (c *Controller) maybeUpdateObjectStatus(ctx context.Context, fw *v1alpha1.FileWatch, newStatus *v1alpha1.FileWatchStatus) error {
	if apicmp.DeepEqual(newStatus, &fw.Status) {
		return nil
	}

	oldError := fw.Status.Error

	update := fw.DeepCopy()
	update.Status = *newStatus
	err := c.Client.Status().Update(ctx, update)
	if err != nil {
		return err
	}

	if update.Status.Error != "" && oldError != update.Status.Error {
		logger.Get(ctx).Errorf("filewatch %s: %s", fw.Name, update.Status.Error)
	}

	c.Store.Dispatch(NewFileWatchUpdateStatusAction(update))
	return nil
}

func (c *Controller) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FileWatch{}).
		Watches(&v1alpha1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc((c.indexer.Enqueue))).
		WatchesRawSource(c.requeuer)

	return b, nil
}

// removeWatch removes a watch from the map. It does NOT stop the watcher or free up resources.
//
// mu must be held before calling.
func (c *Controller) removeWatch(tw *watcher) {
	if entry, ok := c.targetWatches[tw.name]; ok && tw == entry {
		delete(c.targetWatches, tw.name)
	}
}

func (c *Controller) addOrReplace(ctx context.Context, name types.NamespacedName, fw *v1alpha1.FileWatch) {
	existing, hasExisting := c.targetWatches[name]
	status := &v1alpha1.FileWatchStatus{}
	w := &watcher{
		name:           name,
		spec:           *fw.Spec.DeepCopy(),
		clock:          c.clock,
		restartBackoff: time.Second,
	}
	if hasExisting && apicmp.DeepEqual(existing.spec, w.spec) {
		w.restartBackoff = existing.restartBackoff
		status.Error = existing.status.Error
	}
	if hasExisting {
		status.FileEvents = existing.status.FileEvents
		status.LastEventTime = existing.status.LastEventTime
	}

	ignoreMatcher := ignore.CreateFileChangeFilter(fw.Spec.Ignores)
	startFileChangeLoop := false
	notify, err := c.fsWatcherMaker(
		append([]string{}, fw.Spec.WatchedPaths...),
		ignoreMatcher,
		logger.Get(ctx))
	if err != nil {
		status.Error = fmt.Sprintf("filewatch init: %v", err)
	} else if err := notify.Start(); err != nil {
		status.Error = fmt.Sprintf("filewatch init: %v", err)

		// Close the notify immediately, but don't add it to the watcher object. The
		// watcher object is still needed to handle backoff.
		_ = notify.Close()
	} else {
		startFileChangeLoop = true
	}

	if hasExisting {
		// Clean up the existing watch AFTER the new watch has been started.
		existing.cleanupWatch(ctx)
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	if startFileChangeLoop {
		w.notify = notify
		status.MonitorStartTime = apis.NowMicro()
		go c.dispatchFileChangesLoop(ctx, w)
	}

	w.status = status
	c.targetWatches[name] = w
}

func (c *Controller) dispatchFileChangesLoop(ctx context.Context, w *watcher) {
	eventsCh := fsevent.Coalesce(c.timerMaker, w.notify.Events())

	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		w.cleanupWatch(ctx)
		c.requeuer.Add(w.name)
	}()

	for {
		select {
		case err, ok := <-w.notify.Errors():
			if !ok {
				return
			}

			if watch.IsWindowsShortReadError(err) {
				w.recordError(fmt.Errorf("Windows I/O overflow.\n"+
					"You may be able to fix this by setting the env var %s.\n"+
					"Current buffer size: %d\n"+
					"More details: https://github.com/tilt-dev/tilt/issues/3556\n"+
					"Caused by: %v",
					watch.WindowsBufferSizeEnvVar,
					watch.DesiredWindowsBufferSize(),
					err))
			} else if err.Error() == fsnotify.ErrEventOverflow.Error() {
				w.recordError(fmt.Errorf("%s\nerror: %v", DetectedOverflowErrMsg, err))
			} else {
				w.recordError(err)
			}
			c.requeuer.Add(w.name)

		case <-ctx.Done():
			return
		case fsEvents, ok := <-eventsCh:
			if !ok {
				return
			}
			w.recordEvent(fsEvents)
			c.requeuer.Add(w.name)
		}
	}
}

// Find all the objects to watch based on the Filewatch model
func indexFw(obj ctrlclient.Object) []indexer.Key {
	fw := obj.(*v1alpha1.FileWatch)
	result := []indexer.Key{}

	if fw.Spec.DisableSource != nil {
		cm := fw.Spec.DisableSource.ConfigMap
		if cm != nil {
			gvk := v1alpha1.SchemeGroupVersion.WithKind("ConfigMap")
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: cm.Name},
				GVK:  gvk,
			})
		}
	}

	return result
}
