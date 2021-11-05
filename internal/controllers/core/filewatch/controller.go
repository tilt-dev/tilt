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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"

	"github.com/tilt-dev/fsnotify"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch/fsevent"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/filewatches"
	"github.com/tilt-dev/tilt/internal/watch"
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
	indexer        *indexer.Indexer
}

func NewController(client ctrlclient.Client, store store.RStore, fsWatcherMaker fsevent.WatcherMaker, timerMaker fsevent.TimerMaker, scheme *runtime.Scheme) *Controller {
	return &Controller{
		Client:         client,
		Store:          store,
		targetWatches:  make(map[types.NamespacedName]*watcher),
		fsWatcherMaker: fsWatcherMaker,
		timerMaker:     timerMaker,
		indexer:        indexer.NewIndexer(scheme, indexFw),
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

	// Get configmap's disable status
	disableStatus, err := configmap.MaybeNewDisableStatus(ctx, c.Client, fw.Spec.DisableSource, fw.Status.DisableStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update filewatch's disable status
	if disableStatus != fw.Status.DisableStatus {
		fw.Status.DisableStatus = disableStatus
		if err := c.Client.Status().Update(ctx, &fw); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Clean up existing filewatches if it's disabled
	if disableStatus.Disabled {
		if hasExisting {
			existing.cleanupWatch(ctx)
			c.removeWatch(existing)
		}
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	c.Store.Dispatch(filewatches.NewFileWatchUpsertAction(&fw))

	if !hasExisting || !equality.Semantic.DeepEqual(existing.spec, fw.Spec) {
		if err := c.addOrReplace(ctx, c.Store, req.NamespacedName, &fw); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create/update filesystem watch: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *Controller) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FileWatch{}).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc((c.indexer.Enqueue)))

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

func (c *Controller) addOrReplace(ctx context.Context, st store.RStore, name types.NamespacedName, fw *v1alpha1.FileWatch) error {
	ignoreMatcher, err := ignore.IgnoresToMatcher(fw.Spec.Ignores)
	if err != nil {
		return err
	}
	notify, err := c.fsWatcherMaker(
		append([]string{}, fw.Spec.WatchedPaths...),
		ignoreMatcher,
		logger.Get(ctx))
	if err != nil {
		return fmt.Errorf("failed to initialize filesystem watch: %v", err)
	}
	if err := notify.Start(); err != nil {
		return fmt.Errorf("failed to initialize filesystem watch: %v", err)
	}

	// Clear out any old events
	fw.Status.FileEvents = nil
	fw.Status.LastEventTime = metav1.MicroTime{}
	fw.Status.MonitorStartTime = metav1.NowMicro()
	fw.Status.Error = ""

	if err := c.Client.Status().Update(ctx, fw); err != nil {
		_ = notify.Close()
		return fmt.Errorf("failed to update monitor start time: %v", err)
	}
	c.Store.Dispatch(NewFileWatchUpdateStatusAction(fw))

	ctx, cancel := context.WithCancel(ctx)
	w := &watcher{
		name:   name,
		spec:   *fw.Spec.DeepCopy(),
		status: fw.Status.DeepCopy(),
		notify: notify,
		cancel: cancel,
	}

	go c.dispatchFileChangesLoop(ctx, st, w)

	if existing, ok := c.targetWatches[name]; ok {
		// no need to remove from map, will be overwritten
		existing.cleanupWatch(ctx)
	}

	c.targetWatches[name] = w
	return nil
}

func (c *Controller) dispatchFileChangesLoop(ctx context.Context, st store.RStore, w *watcher) {
	eventsCh := fsevent.Coalesce(c.timerMaker, w.notify.Events())

	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		w.cleanupWatch(ctx)
		c.removeWatch(w)
	}()

	for {
		select {
		case err, ok := <-w.notify.Errors():
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
			if err := w.recordEvent(ctx, c.Client, st, fsEvents); err != nil {
				if ctx.Err() == nil {
					// there's an unavoidable race here - the context might have
					// been canceled while we were recording the event, which will
					// cause a failure, so we just ignore _any_ errors in this case
					// (even if it was a non-context related error, this watcher is
					// being disposed of, so it's no longer relevant)
					st.Dispatch(store.NewErrorAction(err))
				} else {
					logger.Get(ctx).Debugf("Ignored stale error for %q: %v", w.name, err)
				}
				return
			}
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
