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

package core

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// FileWatchController reconciles a FileWatch object
type FileWatchController struct {
	ctrlclient.Client
	Store store.RStore
}

func NewFileWatchController(store store.RStore) *FileWatchController {
	return &FileWatchController{
		Store: store,
	}
}

// +kubebuilder:rbac:groups=,resources=filewatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=filewatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=filewatches/finalizers,verbs=update

func (r *FileWatchController) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	// this is currently a no-op stub
	return ctrl.Result{}, nil
}

func (r *FileWatchController) SetClient(client ctrlclient.Client) {
	r.Client = client
}

func (r *FileWatchController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&filewatches.FileWatch{}).
		Owns(&filewatches.FileWatch{}).
		Complete(r)
}
