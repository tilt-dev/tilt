package portforward

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Reconciler struct {
	store      store.RStore
	kClient    k8s.Client
	ctrlClient ctrlclient.Client

	// map of PortForward object name --> running forward(s)
	activeForwards map[types.NamespacedName]portForwardEntry
}

var _ store.TearDowner = &Reconciler{}
var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(store store.RStore, kClient k8s.Client) *Reconciler {
	return &Reconciler{
		store:          store,
		kClient:        kClient,
		activeForwards: make(map[types.NamespacedName]portForwardEntry),
	}
}

func (r *Reconciler) SetClient(client ctrlclient.Client) {
	r.ctrlClient = client
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&PortForward{}).Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	err := r.reconcile(ctx, req.NamespacedName)
	return ctrl.Result{}, err
}

func (r *Reconciler) reconcile(ctx context.Context, name types.NamespacedName) error {
	pf := &PortForward{}
	err := r.ctrlClient.Get(ctx, name, pf)

	if apierrors.IsNotFound(err) || pf.ObjectMeta.DeletionTimestamp != nil {
		// PortForward deleted in API server -- stop and remove it
		r.stop(name)
		return nil
	}

	if active, ok := r.activeForwards[name]; ok {
		if equality.Semantic.DeepEqual(active.Spec, pf.Spec) &&
			equality.Semantic.DeepEqual(active.ObjectMeta.Annotations[v1alpha1.AnnotationManifest],
				pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest]) {
			// Nothing has changed, nothing to do
			return nil
		}

		// An update to a PortForward we're already running -- stop the existing one
		r.stop(name)
	}

	// Create a new PortForward OR recreate a modified PortForward (stopped above)
	entry := newEntry(ctx, pf)
	r.activeForwards[name] = entry

	// Treat port-forwarding errors as part of the pod log
	ctx = store.MustObjectLogHandler(entry.ctx, r.store, entry.PortForward)

	for _, forward := range entry.Spec.Forwards {
		entry := entry
		forward := forward
		go r.startPortForwardLoop(ctx, entry, forward)
	}

	return nil
}

func (r *Reconciler) startPortForwardLoop(ctx context.Context, entry portForwardEntry, forward Forward) {
	originalBackoff := wait.Backoff{
		Steps:    1000,
		Duration: 50 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Cap:      15 * time.Second,
	}
	currentBackoff := originalBackoff

	for {
		start := time.Now()
		err := r.onePortForward(ctx, entry, forward)
		if ctx.Err() != nil {
			// If the context was canceled, we're satisfied.
			// Ignore any errors.
			// TODO: update status with "context cancelled"
			return
		}

		// Otherwise, repeat the loop, maybe logging the error
		if err != nil {
			logger.Get(ctx).Infof("Reconnecting... Error port-forwarding %s (%d -> %d): %v",
				entry.ObjectMeta.Annotations[v1alpha1.AnnotationManifest],
				forward.LocalPort, forward.ContainerPort, err)
			// TODO: if not wasBackoff:
			//   wasBackoff = True
			//   update status = backoff
		}

		// If this failed in less than a second, then we should advance the backoff.
		// Otherwise, reset the backoff.
		if time.Since(start) < time.Second {
			time.Sleep(currentBackoff.Step())
		} else {
			currentBackoff = originalBackoff
		}
	}
}

func (r *Reconciler) onePortForward(ctx context.Context, entry portForwardEntry, forward Forward) error {
	ns := k8s.Namespace(entry.Spec.Namespace)
	podID := k8s.PodID(entry.Spec.PodName)

	pf, err := r.kClient.CreatePortForwarder(ctx, ns, podID, int(forward.LocalPort), int(forward.ContainerPort), forward.Host)
	if err != nil {
		return err
	}

	go func() {
		if pf.ReadyCh() == nil {
			return
		}
		select {
		case err := <-pf.ReadyCh():
			if err == nil {
				fmt.Println("✨ hooray it's ready")
				// TODO: update status = running
			} else {
				// otherwise, if there's an error, we update the status for it
				// one level up
			}
		}
	}()
	err = pf.ForwardPorts()
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) TearDown(ctx context.Context) {
	for name := range r.activeForwards {
		r.stop(name)
	}
}

func (r *Reconciler) stop(name types.NamespacedName) {
	entry, ok := r.activeForwards[name]
	if !ok {
		return
	}
	entry.cancel()
	delete(r.activeForwards, name)
}

type portForwardEntry struct {
	*PortForward
	ctx    context.Context
	cancel func()
}

func newEntry(ctx context.Context, pf *PortForward) portForwardEntry {
	ctx, cancel := context.WithCancel(ctx)
	return portForwardEntry{
		PortForward: pf,
		ctx:         ctx,
		cancel:      cancel,
	}
}
