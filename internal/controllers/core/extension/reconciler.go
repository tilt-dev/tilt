package extension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Reconciler struct {
	ctrlClient ctrlclient.Client
	indexer    *indexer.Indexer
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ExtensionRepo{})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		ctrlClient: ctrlClient,
		indexer:    indexer.NewIndexer(scheme, indexExtension),
	}
}

// Downloads extension repos.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var ext v1alpha1.Extension
	err := r.ctrlClient.Get(ctx, nn, &ext)
	r.indexer.OnReconcile(nn, &ext)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Cleanup tiltfile loads if an extension is deleted.
	if apierrors.IsNotFound(err) || !ext.ObjectMeta.DeletionTimestamp.IsZero() {
		err := r.manageOwnedTiltfile(ctx, nn, nil)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var repo v1alpha1.ExtensionRepo
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: ext.Spec.RepoName}, &repo)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) {
		return r.updateError(ctx, &ext, fmt.Sprintf("extension repo %s not found", ext.Spec.RepoName))
	}

	if repo.Status.Path == "" {
		return r.updateError(ctx, &ext, fmt.Sprintf("extension repo %s not loaded yet", ext.Spec.RepoName))
	}

	absPath := filepath.Join(repo.Status.Path, ext.Spec.RepoPath, "Tiltfile")

	// Make sure the user isn't trying to use path tricks to "escape" the repo.
	if !ospath.IsChild(repo.Status.Path, absPath) {
		return r.updateError(ctx, &ext, fmt.Sprintf("invalid repo path: %s", ext.Spec.RepoPath))
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.Mode().IsRegular() {
		return r.updateError(ctx, &ext, fmt.Sprintf("no extension tiltfile found at %s", absPath))
	}

	// TODO(nick): Create Tiltfile child object.
	return r.updateStatus(ctx, &ext, func(status *v1alpha1.ExtensionStatus) {
		status.Path = absPath
		status.Error = ""
	})
}

// Generic status update.
func (r *Reconciler) updateStatus(ctx context.Context, ext *v1alpha1.Extension, mutateFn func(*v1alpha1.ExtensionStatus)) (ctrl.Result, error) {
	update := ext.DeepCopy()
	mutateFn(&(update.Status))

	if apicmp.DeepEqual(update.Status, ext.Status) {
		return ctrl.Result{}, nil
	}
	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.manageOwnedTiltfile(ctx, types.NamespacedName{Name: update.Name}, update)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// Update status with an error message, logging the error.
func (r *Reconciler) updateError(ctx context.Context, ext *v1alpha1.Extension, errorMsg string) (ctrl.Result, error) {
	update := ext.DeepCopy()
	update.Status.Error = errorMsg
	update.Status.Path = ""

	if apicmp.DeepEqual(update.Status, ext.Status) {
		return ctrl.Result{}, nil
	}

	logger.Get(ctx).Errorf("extension %s: %s", ext.Name, errorMsg)

	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.manageOwnedTiltfile(ctx, types.NamespacedName{Name: update.Name}, update)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// Find all the objects we need to watch based on the extension spec.
func indexExtension(obj client.Object) []indexer.Key {
	result := []indexer.Key{}
	ext := obj.(*v1alpha1.Extension)
	if ext.Spec.RepoName != "" {
		repoGVK := v1alpha1.SchemeGroupVersion.WithKind("ExtensionRepo")
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: ext.Spec.RepoName},
			GVK:  repoGVK,
		})
	}
	return result
}
