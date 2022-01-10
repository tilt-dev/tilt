package cmdimage

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Manages the CmdImage API object.
type Reconciler struct {
	client ctrlclient.Client
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client) *Reconciler {
	return &Reconciler{
		client: client,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &v1alpha1.CmdImage{}
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || obj.ObjectMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CmdImage{})

	return b, nil
}
