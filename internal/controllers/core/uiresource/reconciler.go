package uiresource

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/uiresources"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// The uiresource.Reconciler is not a real reconciler because UIResource is not
// a real API object.
//
// It's a fake status object that reports the Status of the legacy engine. The
// uiresource.Reconciler wathces that status and broadcasts it to the legacy web
// UI.
type Reconciler struct {
	client ctrlclient.Client
	wsList *server.WebsocketList
	store  store.RStore
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, wsList *server.WebsocketList, store store.RStore) *Reconciler {
	return &Reconciler{
		client: client,
		wsList: wsList,
		store:  store,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	resource := &v1alpha1.UIResource{}
	err := r.client.Get(ctx, req.NamespacedName, resource)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("uiresource reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || resource.ObjectMeta.DeletionTimestamp != nil {
		r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
			ws.SendUIResourceUpdate(ctx, req.NamespacedName, nil)
		})

		r.store.Dispatch(uiresources.NewUIResourceDeleteAction(req.Name))
		return ctrl.Result{}, nil
	}

	r.store.Dispatch(uiresources.NewUIResourceUpsertAction(resource))

	r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
		ws.SendUIResourceUpdate(ctx, req.NamespacedName, resource)
	})

	return ctrl.Result{}, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.UIResource{})

	return b, nil
}
