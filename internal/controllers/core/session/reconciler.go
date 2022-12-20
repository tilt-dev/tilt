package session

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/sessions"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Session reports on current CI/Up state, and determines
// when Tilt should exit.
//
// Reads the Session Spec and updates the Session Status.
//
// Dispatches an event to the store for handling exits.
type Reconciler struct {
	client   ctrlclient.Client
	st       store.RStore
	requeuer *indexer.Requeuer
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, st store.RStore) *Reconciler {
	return &Reconciler{
		client:   client,
		st:       st,
		requeuer: indexer.NewRequeuer(),
	}
}

func (r *Reconciler) Requeue() {
	r.requeuer.Add(types.NamespacedName{Name: sessions.DefaultSessionName})
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	session := &v1alpha1.Session{}
	err := r.client.Get(ctx, req.NamespacedName, session)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("session reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || session.ObjectMeta.DeletionTimestamp != nil {
		// NOTE(nick): This should never happen, and if it does, Tilt should
		// immediately re-create the session.
		return ctrl.Result{}, nil
	}

	r.st.Dispatch(sessions.NewSessionUpsertAction(session))

	err = r.maybeUpdateObjectStatus(ctx, session)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// maybeUpdateObjectStatus builds the latest status for the Session and persists it.
// Should only be called in the main reconciler thread.
//
// If the status has not changed since the last status update performed (by the
// Reconciler), it will be skipped.
//
// Returns the latest object on success.
func (r *Reconciler) maybeUpdateObjectStatus(ctx context.Context, session *v1alpha1.Session) error {
	status := r.makeLatestStatus(session)
	if apicmp.DeepEqual(session.Status, status) {
		// the status hasn't changed - avoid a spurious update
		return nil
	}

	update := session.DeepCopy()
	update.Status = status
	err := r.client.Status().Update(ctx, update)
	if err != nil {
		return err
	}
	r.st.Dispatch(sessions.NewSessionUpsertAction(update))
	return nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Session{}).
		Watches(r.requeuer, handler.Funcs{})

	return b, nil
}
