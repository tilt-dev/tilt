package uibutton

import (
	"context"
	"fmt"

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

func NewReconciler(wsList *server.WebsocketList) *Reconciler {
	return &Reconciler{wsList: wsList}
}

func (r *Reconciler) SetClient(client ctrlclient.Client) {
	r.client = client
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

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.UIButton{}).
		Complete(r)
}
