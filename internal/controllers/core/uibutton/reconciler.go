package uibutton

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Reconciler struct {
	client ctrlclient.Client
	wsList *server.WebsocketList
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, wsList *server.WebsocketList) *Reconciler {
	return &Reconciler{
		client: client,
		wsList: wsList,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	button := &v1alpha1.UIButton{}
	err := r.client.Get(ctx, req.NamespacedName, button)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("uibutton reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || button.ObjectMeta.DeletionTimestamp != nil {
		r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
			ws.SendUIButtonUpdate(ctx, req.NamespacedName, nil)
		})

		return ctrl.Result{}, nil
	}

	r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
		ws.SendUIButtonUpdate(ctx, req.NamespacedName, button)
	})

	return ctrl.Result{}, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.UIButton{})

	return b, nil
}
