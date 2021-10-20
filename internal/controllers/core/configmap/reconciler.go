package configmap

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/configmaps"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Reconciler struct {
	client ctrlclient.Client
	store  store.RStore
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, store store.RStore) *Reconciler {
	return &Reconciler{
		client: client,
		store:  store,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cm := &v1alpha1.ConfigMap{}
	err := r.client.Get(ctx, req.NamespacedName, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || cm.ObjectMeta.DeletionTimestamp != nil {
		r.store.Dispatch(configmaps.NewConfigMapDeleteAction(req.Name))
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.store.Dispatch(configmaps.NewConfigMapUpsertAction(cm))

	return ctrl.Result{}, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ConfigMap{})

	return b, nil
}
