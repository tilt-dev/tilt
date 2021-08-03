package extensionrepo

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Reconciler struct {
	ctrlClient ctrlclient.Client
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ExtensionRepo{})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client) *Reconciler {
	return &Reconciler{
		ctrlClient: ctrlClient,
	}
}

// Downloads extension repos.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var repo v1alpha1.ExtensionRepo
	err := r.ctrlClient.Get(ctx, nn, &repo)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !repo.ObjectMeta.DeletionTimestamp.IsZero() {
		// TODO(nick): Handle deletion
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}
