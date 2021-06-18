package kubernetesapply

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/k8s"
)

type Reconciler struct {
	kCli       k8s.Client
	ctrlClient ctrlclient.Client
}

func (w *Reconciler) SetClient(client ctrlclient.Client) {
	w.ctrlClient = client
}

func (w *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesApply{}).
		Complete(w)
}

func NewReconciler(kCli k8s.Client) *Reconciler {
	return &Reconciler{
		kCli: kCli,
	}
}

// Reconcile manages namespace watches for the modified KubernetesApply object.
func (w *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var kd v1alpha1.KubernetesApply
	err := w.ctrlClient.Get(ctx, request.NamespacedName, &kd)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !kd.ObjectMeta.DeletionTimestamp.IsZero() {
		// delete
		return ctrl.Result{}, nil
	}

	// add or replace

	return ctrl.Result{}, nil
}
