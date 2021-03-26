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

	"github.com/tilt-dev/tilt/internal/engine/fswatch"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch/fsevent"

	"github.com/tilt-dev/fsnotify"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/logger"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Controller reconciles a FileWatch object
type Controller struct {
	ctrlclient.Client
	Store store.RStore

	targetWatches  map[types.NamespacedName]*watcher
	fsWatcherMaker fsevent.WatcherMaker
	timerMaker     fsevent.TimerMaker
	mu             sync.Mutex
}

func NewController(store store.RStore, fsWatcherMaker fsevent.WatcherMaker, timerMaker fsevent.TimerMaker) *Controller {
	return &Controller{
		Store:          store,
		targetWatches:  make(map[types.NamespacedName]*watcher),
		fsWatcherMaker: fsWatcherMaker,
		timerMaker:     timerMaker,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	existing, hasExisting := c.targetWatches[req.NamespacedName]

	var fw filewatches.FileWatch
	err := c.Client.Get(ctx, req.NamespacedName, &fw)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !fw.ObjectMeta.DeletionTimestamp.IsZero() {
		if hasExisting {
			existing.cleanupWatch(ctx)
			c.removeWatch(existing)
		}
		return ctrl.Result{}, nil
	}

	if !hasExisting || !equality.Semantic.DeepEqual(existing.spec, fw.Spec) {
		if err := c.addOrReplace(ctx, c.Store, req.NamespacedName, &fw); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create/update filesystem watch: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *Controller) SetClient(client ctrlclient.Client) {
	c.Client = client
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&filewatches.FileWatch{}).
		Complete(c)
}

// removeWatch removes a watch from the map. It does NOT stop the watcher or free up resources.
//
// mu must be held before calling.
func (c *Controller) removeWatch(tw *watcher) {
	if entry, ok := c.targetWatches[tw.name]; ok && tw == entry {
		delete(c.targetWatches, tw.name)
	}
}

func (c *Controller) addOrReplace(ctx context.Context, st store.RStore, name types.NamespacedName, fw *filewatches.FileWatch) error {
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

	// replace the entirety of status to clear out any old events
	fw.Status = filewatches.FileWatchStatus{
		MonitorStartTime: metav1.NowMicro(),
	}
	if err := c.Client.Status().Update(ctx, fw); err != nil {
		_ = notify.Close()
		return fmt.Errorf("failed to update monitor start time: %v", err)
	}
	c.Store.Dispatch(fswatch.NewFileWatchUpdateStatusAction(fw))

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
				st.Dispatch(store.NewErrorAction(err))
				return
			}
		}
	}
}
