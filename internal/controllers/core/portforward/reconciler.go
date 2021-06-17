package portforward

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/logger"

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
)

type Reconciler struct {
	store      store.RStore
	kClient    k8s.Client
	ctrlClient ctrlclient.Client

	// map of PortForward object name --> running forward(s)
	activeForwards map[types.NamespacedName]*portForwardEntry
}

var _ store.TearDowner = &Reconciler{}
var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(store store.RStore, kClient k8s.Client) *Reconciler {
	return &Reconciler{
		store:          store,
		kClient:        kClient,
		activeForwards: make(map[types.NamespacedName]*portForwardEntry),
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
		go r.portForwardLoop(ctx, entry, forward)
	}

	return nil
}

func (r *Reconciler) portForwardLoop(ctx context.Context, entry *portForwardEntry, forward Forward) {
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
		r.onePortForward(ctx, entry, forward)
		if ctx.Err() != nil {
			// If the context was canceled, there's nothing more to do;
			// we cannot even update the status because we no longer have
			// a valid context, but that's fine because that means this
			// PortForward is being deleted.
			return
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

func (r *Reconciler) updateForwardStatus(ctx context.Context, entry *portForwardEntry) {
	var pf v1alpha1.PortForward
	key := apis.Key(entry.PortForward)
	if err := r.ctrlClient.Get(ctx, key, &pf); err != nil {
		if !apierrors.IsNotFound(err) {
			// short of dispatching a fatal error, there's nothing that can really be done here, so just log it
			// for debugging purposes
			logger.Get(ctx).Debugf("Failed to fetch PortForward %q for status update: %v", entry.Name, err)
		}
		return
	}

	newStatuses := entry.statuses()
	if equality.Semantic.DeepEqual(pf.Status.ForwardStatuses, newStatuses) {
		// the forwards didn't actually change, so skip the update
		return
	}

	pf.Status.ForwardStatuses = newStatuses
	if err := r.ctrlClient.Status().Update(ctx, &pf); err != nil {
		if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			logger.Get(ctx).Debugf("Failed to update status for PortForward %q: %v", entry.Name, err)
		}
	}
}

func (r *Reconciler) onePortForward(ctx context.Context, entry *portForwardEntry, forward Forward) {
	logError := func(err error) {
		logger.Get(ctx).Infof("Reconnecting... Error port-forwarding %s (%d -> %d): %v",
			entry.ObjectMeta.Annotations[v1alpha1.AnnotationManifest],
			forward.LocalPort, forward.ContainerPort, err)
	}

	pf, err := r.kClient.CreatePortForwarder(
		ctx,
		k8s.Namespace(entry.Spec.Namespace),
		k8s.PodID(entry.Spec.PodName),
		int(forward.LocalPort),
		int(forward.ContainerPort),
		forward.Host)
	if err != nil {
		logError(err)
		shouldUpdate := entry.setStatus(forward, ForwardStatus{
			LocalPort:     forward.LocalPort,
			ContainerPort: forward.ContainerPort,
			Error:         err.Error(),
		})
		if shouldUpdate {
			r.updateForwardStatus(ctx, entry)
		}
		return
	}

	// wait in the background for the port forwarder to signal that it's ready to update the status
	// the doneCh ensures we don't leak the goroutine if ForwardPorts() errors out early without
	// ever becoming ready
	doneCh := make(chan struct{}, 1)
	go func() {
		readyCh := pf.ReadyCh()
		if readyCh == nil {
			return
		}
		select {
		case <-doneCh:
			return
		case <-readyCh:
			entry.setStatus(forward, ForwardStatus{
				LocalPort:     int32(pf.LocalPort()),
				ContainerPort: forward.ContainerPort,
				Addresses:     pf.Addresses(),
				StartedAt:     apis.NowMicro(),
			})
			r.updateForwardStatus(ctx, entry)
		}
	}()

	err = pf.ForwardPorts()
	close(doneCh)
	if err != nil {
		logError(err)
		shouldUpdate := entry.setStatus(forward, ForwardStatus{
			LocalPort:     int32(pf.LocalPort()),
			ContainerPort: forward.ContainerPort,
			Addresses:     pf.Addresses(),
			Error:         err.Error(),
		})
		if shouldUpdate {
			r.updateForwardStatus(ctx, entry)
		}
		return
	}
}

func (r *Reconciler) TearDown(_ context.Context) {
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

	mu     sync.Mutex
	status map[Forward]statusMeta
}

func newEntry(ctx context.Context, pf *PortForward) *portForwardEntry {
	ctx, cancel := context.WithCancel(ctx)
	return &portForwardEntry{
		PortForward: pf,
		ctx:         ctx,
		cancel:      cancel,
		status:      make(map[Forward]statusMeta),
	}
}

type statusMeta struct {
	status    ForwardStatus
	lastError time.Time
}

// setStatus tracks the latest status for this Forward and returns a bool indicating whether
// an API update should be performed.
func (e *portForwardEntry) setStatus(spec Forward, status ForwardStatus) (shouldUpdate bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var lastError time.Time
	if status.Error != "" {
		lastError = time.Now()
		// if this port forward last failed more than a second ago (or had lastError reset
		// by having gone into a success status), do an update
		shouldUpdate = e.status[spec].lastError.Before(time.Now().Add(-time.Second))
	} else {
		// always update on success
		shouldUpdate = true
	}

	e.status[spec] = statusMeta{
		status:    status,
		lastError: lastError,
	}
	return shouldUpdate
}

func (e *portForwardEntry) statuses() []ForwardStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	var statuses []ForwardStatus
	for _, s := range e.status {
		statuses = append(statuses, *s.status.DeepCopy())
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		if statuses[i].ContainerPort < statuses[j].ContainerPort {
			return true
		}
		if statuses[i].LocalPort < statuses[j].LocalPort {
			return true
		}
		return timecmp.BeforeOrEqual(statuses[i].StartedAt, statuses[j].StartedAt)
	})
	return statuses
}
