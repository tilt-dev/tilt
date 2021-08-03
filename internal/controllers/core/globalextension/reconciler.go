package globalextension

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

	var ext v1alpha1.GlobalExtension
	err := r.ctrlClient.Get(ctx, nn, &ext)
	r.indexer.OnReconcile(nn, &ext)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !ext.ObjectMeta.DeletionTimestamp.IsZero() {
		// TODO(nick): Handle deletion
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// Find all the objects we need to watch based on the extension spec.
func indexExtension(obj client.Object) []indexer.Key {
	result := []indexer.Key{}
	ext := obj.(*v1alpha1.GlobalExtension)
	if ext.Spec.RepoName != "" {
		repoGVK := v1alpha1.SchemeGroupVersion.WithKind("ExtensionRepo")
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: ext.Spec.RepoName},
			GVK:  repoGVK,
		})
	}
	return result
}
