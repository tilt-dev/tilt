package extension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Reconciler struct {
	ctrlClient ctrlclient.Client
	indexer    *indexer.Indexer
	mu         sync.Mutex
	analytics  *analytics.TiltAnalytics
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Extension{}).
		Watches(&source.Kind{Type: &v1alpha1.ExtensionRepo{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, scheme *runtime.Scheme, analytics *analytics.TiltAnalytics) *Reconciler {
	return &Reconciler{
		ctrlClient: ctrlClient,
		indexer:    indexer.NewIndexer(scheme, indexExtension),
		analytics:  analytics,
	}
}

// Verifies extension paths.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

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

	newStatus := r.apply(&ext, &repo)

	update, changed, err := r.maybeUpdateStatus(ctx, &ext, newStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always manage the child objects, even if the user-visible status didn't change,
	// because there might be internal state we need to propagate.
	err = r.manageOwnedTiltfile(ctx, types.NamespacedName{Name: ext.Name}, update)
	if err != nil {
		return ctrl.Result{}, err
	}

	if changed && update.Status.Error == "" {
		repoType := "http"
		if strings.HasPrefix(repo.Spec.URL, "file://") {
			repoType = "file"
		}
		r.analytics.Incr("api.extension.load", map[string]string{
			"ext_path":      ext.Spec.RepoPath,
			"repo_url_hash": analytics.HashSHA1(repo.Spec.URL),
			"repo_type":     repoType,
		})
	}

	return ctrl.Result{}, nil
}

// Reconciles the extension without reading or writing from the API server.
// Returns the resolved status.
// Exposed for outside callers.
func (r *Reconciler) ForceApply(ext *v1alpha1.Extension, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.apply(ext, repo)
}

func (r *Reconciler) apply(ext *v1alpha1.Extension, repo *v1alpha1.ExtensionRepo) v1alpha1.ExtensionStatus {
	if repo.Name == "" {
		return v1alpha1.ExtensionStatus{Error: fmt.Sprintf("extension repo %s not found", ext.Spec.RepoName)}
	}

	if repo.Status.Path == "" {
		return v1alpha1.ExtensionStatus{Error: fmt.Sprintf("extension repo %s not loaded yet", ext.Spec.RepoName)}
	}

	absPath := filepath.Join(repo.Status.Path, ext.Spec.RepoPath, "Tiltfile")

	// Make sure the user isn't trying to use path tricks to "escape" the repo.
	if !ospath.IsChild(repo.Status.Path, absPath) {
		return v1alpha1.ExtensionStatus{Error: fmt.Sprintf("invalid repo path: %s", ext.Spec.RepoPath)}
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.Mode().IsRegular() {
		return v1alpha1.ExtensionStatus{Error: fmt.Sprintf("no extension tiltfile found at %s", absPath)}
	}

	return v1alpha1.ExtensionStatus{Path: absPath}
}

// Update the status. Returns true if the status changed.
func (r *Reconciler) maybeUpdateStatus(ctx context.Context, obj *v1alpha1.Extension, newStatus v1alpha1.ExtensionStatus) (*v1alpha1.Extension, bool, error) {
	if apicmp.DeepEqual(obj.Status, newStatus) {
		return obj, false, nil
	}

	oldError := obj.Status.Error
	newError := newStatus.Error
	update := obj.DeepCopy()
	update.Status = *(newStatus.DeepCopy())

	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return obj, false, err
	}

	if newError != "" && oldError != newError {
		logger.Get(ctx).Errorf("extension %s: %s", obj.Name, newError)
	}
	return update, true, err
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
