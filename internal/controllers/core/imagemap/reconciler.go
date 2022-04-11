package imagemap

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/imagemaps"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Reconciler manages the ImageMap API object.
type Reconciler struct {
	client     ctrlclient.Client
	dispatcher store.Dispatcher
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, dispatcher store.Dispatcher) *Reconciler {
	return &Reconciler{
		client:     client,
		dispatcher: dispatcher,
	}
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ImageMap{})

	return b, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj v1alpha1.ImageMap
	err := r.client.Get(ctx, req.NamespacedName, &obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || obj.ObjectMeta.DeletionTimestamp != nil {
		r.dispatcher.Dispatch(imagemaps.NewImageMapDeleteAction(req.Name))
		return ctrl.Result{}, nil
	}

	r.dispatcher.Dispatch(imagemaps.NewImageMapUpsertAction(&obj))
	return ctrl.Result{}, nil
}
