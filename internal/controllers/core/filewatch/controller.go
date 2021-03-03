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
	"path/filepath"
	"sync"

	"github.com/tilt-dev/fsnotify"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const maxFileWatchHistory = 20

// Controller reconciles a filewatches.FileWatch object.
type Controller struct {
	ctrlclient.Client

	watcherMaker watch.FsWatcherMaker
	timerMaker   watch.TimerMaker

	mu      sync.Mutex
	watches map[types.NamespacedName]*fsWatch
}

func NewController(watcherMaker watch.FsWatcherMaker, timerMaker watch.TimerMaker) *Controller {
	return &Controller{
		watcherMaker: watcherMaker,
		timerMaker:   timerMaker,
		watches:      make(map[types.NamespacedName]*fsWatch),
	}
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.Get(ctx).WithFields(logger.Fields{"apiserver_entity": req.NamespacedName.String()})
	ctx = logger.WithLogger(ctx, log)

	var fileWatchApiObj filewatches.FileWatch
	if err := c.Get(ctx, req.NamespacedName, &fileWatchApiObj); err != nil {
		if apierrors.IsNotFound(err) {
			c.StopWatch(req.NamespacedName)
			err = nil
		}
		return ctrl.Result{}, err
	}

	if !fileWatchApiObj.ObjectMeta.DeletionTimestamp.IsZero() {
		// object is being deleted - stop reconcile
		return ctrl.Result{}, nil
	}

	// N.B. the background context is used as the root context for file watching; otherwise, the file watch would
	//		be canceled as soon as reconciliation was done
	fileWatchCtx := logger.WithLogger(context.Background(), log)
	// reconciliation MUST be idempotent; StartWatch() will noop if spec hasn't changed and update it if it has
	addedOrUpdated, err := c.StartWatch(fileWatchCtx, req.NamespacedName, fileWatchApiObj.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}
	if addedOrUpdated {
		now := metav1.Now()
		fileWatchApiObj.Status.LastEventTime = &now

		if err := c.Update(ctx, &fileWatchApiObj); err != nil {
			// remove the watch that was just added/updated; otherwise, when reconcile is called again, it won't
			// be seen as new and won't get properly initialized
			c.StopWatch(req.NamespacedName)
			return ctrl.Result{}, err
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

type fsWatch struct {
	name     types.NamespacedName
	spec     filewatches.FileWatchSpec
	logger   logger.Logger
	notifier watch.Notify

	events []filewatches.FileEvent

	cancel func()
}

func (c *Controller) StopWatch(name types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.watches[name]
	if !ok {
		return
	}

	c.cleanupAndRemoveWatch(w)
}

// StartWatch adds a new filesystem watch for a FileWatch API object.
//
// If a watch for the spec already exists and the spec has not changed, the existing watch is left untouched
// and the boolean return value is false. If there is no watch or the spec has changed, it is replaced and
// the boolean return value is true.
func (c *Controller) StartWatch(ctx context.Context, name types.NamespacedName, spec filewatches.FileWatchSpec) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldWatch := c.watches[name]
	if oldWatch != nil {
		if equality.Semantic.DeepEqual(oldWatch.spec, spec) {
			return false, nil
		}
	}

	var watchPaths []string
	var ignoreMatchers []model.PathMatcher
	for _, watchDef := range spec.Watches {
		absWatchPathsForDef, err := absPaths(watchDef.RootPath, watchDef.Paths)
		if err != nil {
			return false, err
		}
		watchPaths = append(watchPaths, absWatchPathsForDef...)

		ignoreMatcher, err := dockerignore.NewDockerPatternMatcher(watchDef.RootPath, watchDef.IgnorePatterns)
		if err != nil {
			return false, err
		}
		ignoreMatchers = append(ignoreMatchers, ignoreMatcher)
	}
	compositeIgnoreMatcher := model.NewCompositeMatcher(ignoreMatchers)

	notifier, err := c.watcherMaker(watchPaths, compositeIgnoreMatcher, logger.Get(ctx))
	if err != nil {
		return false, err
	}

	if err := notifier.Start(); err != nil {
		return false, err
	}

	watchCtx, cancel := context.WithCancel(ctx)
	w := &fsWatch{
		name:     name,
		spec:     spec,
		logger:   logger.Get(watchCtx),
		notifier: notifier,
		cancel:   cancel,
	}
	go c.notifyLoop(watchCtx, w)

	// the old watch is cleaned up AFTER starting the new one to avoid missing events
	// (this might mean duplicates are received, which is an acceptable trade-off)
	if oldWatch != nil {
		c.cleanupAndRemoveWatch(oldWatch)
	}

	c.watches[name] = w

	return true, nil
}

func (c *Controller) cleanupAndRemoveWatch(w *fsWatch) {
	if err := w.notifier.Close(); err != nil {
		w.logger.Debugf("Error cleaning up FS watch for %q: %v", w.name, err)
	}
	w.cancel()

	entry := c.watches[w.name]
	if entry == w {
		delete(c.watches, w.name)
	}
}

// updateStatus overwrites the status for a FileWatch API object.
//
// To avoid stale writes due to cache, the full status should always be constructed rather than read and mutated.
// (This means that the file watcher needs to hold onto enough state to reconstruct status at any moment.)
func (c *Controller) updateStatus(ctx context.Context, name types.NamespacedName, newStatus *filewatches.FileWatchStatus) error {
	var fwObj filewatches.FileWatch
	if err := c.Get(ctx, name, &fwObj); err != nil {
		return ctrlclient.IgnoreNotFound(err)
	}

	newStatus.DeepCopyInto(&fwObj.Status)

	if err := c.Status().Update(ctx, &fwObj); err != nil {
		return err
	}

	return nil
}

func (c *Controller) notifyLoop(ctx context.Context, w *fsWatch) {
	eventsCh := watch.CoalesceEvents(c.timerMaker, w.notifier.Events())

	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.cleanupAndRemoveWatch(w)
	}()

	for {
		select {
		case fsEvents, ok := <-eventsCh:
			if !ok {
				return
			}

			now := metav1.Now()
			apiFileEvent := filewatches.FileEvent{Time: *now.DeepCopy()}
			for _, e := range fsEvents {
				apiFileEvent.SeenFiles = append(apiFileEvent.SeenFiles, e.Path())
			}

			w.events = append(w.events, apiFileEvent)
			if len(w.events) > maxFileWatchHistory {
				w.events = w.events[len(w.events)-maxFileWatchHistory:]
			}

			newStatus := &filewatches.FileWatchStatus{
				LastEventTime: now.DeepCopy(),
				Error:         "",
				FileEvents:    make([]filewatches.FileEvent, len(w.events)),
			}
			for i, e := range w.events {
				e.DeepCopyInto(&newStatus.FileEvents[i])
			}

			err := c.updateStatus(ctx, w.name, newStatus)
			if err != nil {
				w.logger.Debugf("Failed to record FS events for %q: %v", w.name, err)
			}
		case err, ok := <-w.notifier.Errors():
			if !ok {
				return
			}

			now := metav1.Now()
			var errorMessage string
			if watch.IsWindowsShortReadError(err) {
				errorMessage = watch.WindowsShortReadErrorMessage(err)
			} else if err.Error() == fsnotify.ErrEventOverflow.Error() {
				errorMessage = watch.DetectedOverflowErrorMessage(err)
			} else {
				errorMessage = err.Error()
			}

			newStatus := &filewatches.FileWatchStatus{
				LastEventTime: now.DeepCopy(),
				Error:         errorMessage,
				FileEvents:    nil,
			}

			updateErr := c.updateStatus(ctx, w.name, newStatus)
			if updateErr != nil {
				// TODO(milas): make this log message coherent (also - should this be fatal?)
				w.logger.Debugf("Failed to update FileWatch: %v", updateErr)
			}
		case <-ctx.Done():
			return
		}
	}
}

// absPaths returns absolute paths for all paths or an error if a path that is already absolute is encountered.
func absPaths(rootDir string, paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) {
			return nil, fmt.Errorf("path is not relative: %q", path)
		}
		out = append(out, filepath.Join(rootDir, path))
	}
	return out, nil
}
