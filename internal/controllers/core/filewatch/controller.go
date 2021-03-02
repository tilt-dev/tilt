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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// fsFileWatchFinalizer cleans up the actual filesystem (fs) monitor for a FileWatch spec.
const fsFileWatchFinalizer = "fswatch.filewatch.finalizers.core.tilt.dev"

// Controller reconciles a filewatches.FileWatch object.
type Controller struct {
	ctrlclient.Client

	WatchManager *ApiServerWatchManager
}

func NewController(wm *ApiServerWatchManager) *Controller {
	return &Controller{
		WatchManager: wm,
	}
}

// +kubebuilder:rbac:groups=,resources=filewatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=filewatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=filewatches/finalizers,verbs=update

func (r *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.Get(ctx).WithFields(logger.Fields{"apiserver_entity": req.NamespacedName.String()})
	ctx = logger.WithLogger(ctx, log)

	var fileWatchApiObj filewatches.FileWatch
	if err := r.Get(ctx, req.NamespacedName, &fileWatchApiObj); err != nil {
		return ctrl.Result{}, ctrlclient.IgnoreNotFound(err)
	}

	if fileWatchApiObj.ObjectMeta.DeletionTimestamp.IsZero() {
		// ensure finalizer is attached to non-deleted objects so that the actual filesystem-level
		// monitor is removed upon deletion
		if !controllerutil.ContainsFinalizer(&fileWatchApiObj, fsFileWatchFinalizer) {
			controllerutil.AddFinalizer(&fileWatchApiObj, fsFileWatchFinalizer)
			if err := r.Update(ctx, &fileWatchApiObj); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&fileWatchApiObj, fsFileWatchFinalizer) {
			r.WatchManager.StopWatch(fileWatchApiObj.Name)
			controllerutil.RemoveFinalizer(&fileWatchApiObj, fsFileWatchFinalizer)
			if err := r.Update(ctx, &fileWatchApiObj); err != nil {
				return ctrl.Result{}, err
			}
		}

		// object is being deleted - stop reconcile
		return ctrl.Result{}, nil
	}

	// N.B. the background context is used as the root context for file watching; otherwise, the file watch would
	//		be canceled as soon as reconciliation was done
	fileWatchCtx := logger.WithLogger(context.Background(), log)
	// reconciliation MUST be idempotent; StartWatch() will noop if spec hasn't changed and update it if it has
	addedOrUpdated, err := r.WatchManager.StartWatch(fileWatchCtx, fileWatchApiObj.Name, fileWatchApiObj.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}
	if addedOrUpdated {
		log.Debugf("Added/updated FS watch for FileWatch API object")
		// TODO(milas): revisit this logic based on finalized data flows (e.g. does it make more sense to
		//				simply reset LastEventTime + SeenFiles to nil?
		now := metav1.Now()
		fileWatchApiObj.Status.LastEventTime = &now
		fileWatchApiObj.Status.SeenFiles = nil

		if err := r.Update(ctx, &fileWatchApiObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *Controller) SetClient(client ctrlclient.Client) {
	r.Client = client
}

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&filewatches.FileWatch{}).
		Complete(r)
}
