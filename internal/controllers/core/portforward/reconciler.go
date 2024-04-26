package portforward

import (
	"context"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
)

var clusterGVK = v1alpha1.SchemeGroupVersion.WithKind("Cluster")

type Reconciler struct {
	store      store.RStore
	ctrlClient ctrlclient.Client
	clients    *cluster.ClientManager
	requeuer   *indexer.Requeuer
	indexer    *indexer.Indexer

	// map of PortForward object name --> running forward(s)
	activeForwards map[types.NamespacedName]*portForwardEntry
}

var _ store.TearDowner = &Reconciler{}
var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(
	ctrlClient ctrlclient.Client,
	scheme *runtime.Scheme,
	store store.RStore,
	clients cluster.ClientProvider,
) *Reconciler {
	return &Reconciler{
		store:          store,
		ctrlClient:     ctrlClient,
		clients:        cluster.NewClientManager(clients),
		requeuer:       indexer.NewRequeuer(),
		indexer:        indexer.NewIndexer(scheme, indexPortForward),
		activeForwards: make(map[types.NamespacedName]*portForwardEntry),
	}
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&PortForward{}).
		WatchesRawSource(r.requeuer)

	return b, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	err := r.reconcile(ctx, req.NamespacedName)
	return ctrl.Result{}, err
}

func (r *Reconciler) reconcile(ctx context.Context, name types.NamespacedName) error {
	pf := &PortForward{}
	err := r.ctrlClient.Get(ctx, name, pf)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	r.indexer.OnReconcile(name, pf)
	if apierrors.IsNotFound(err) || pf.ObjectMeta.DeletionTimestamp != nil {
		// PortForward deleted in API server -- stop and remove it
		r.stop(name)
		return nil
	}

	var clusterObj v1alpha1.Cluster
	if err := r.ctrlClient.Get(ctx, clusterNN(pf), &clusterObj); err != nil {
		return err
	}
	clusterUpToDate := !r.clients.Refresh(pf, &clusterObj)

	needsCreate := true
	if active, ok := r.activeForwards[name]; ok {
		if clusterUpToDate &&
			equality.Semantic.DeepEqual(active.spec, pf.Spec) &&
			equality.Semantic.DeepEqual(active.meta.Annotations[v1alpha1.AnnotationManifest],
				pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest]) {

			// No change needed.
			needsCreate = false
		} else {
			// An update to a PortForward we're already running -- stop the existing one
			r.stop(name)
		}
	}

	if needsCreate {
		kCli, err := r.clients.GetK8sClient(pf, &clusterObj)
		if err != nil {
			// TODO(milas): a top-level error field on PortForwardStatus is
			// 	likely warranted to report issues like this
			return err
		}

		// Create a new PortForward OR recreate a modified PortForward (stopped above)
		entry := newEntry(ctx, pf, kCli)
		r.activeForwards[name] = entry

		// Treat port-forwarding errors as part of the pod log
		ctx = store.MustObjectLogHandler(entry.ctx, r.store, pf)

		for _, forward := range entry.spec.Forwards {
			go r.portForwardLoop(ctx, entry, forward)
		}
	}

	return r.maybeUpdateStatus(ctx, pf, r.activeForwards[name])
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

func (r *Reconciler) maybeUpdateStatus(ctx context.Context, pf *v1alpha1.PortForward, entry *portForwardEntry) error {
	newStatuses := entry.statuses()
	if apicmp.DeepEqual(pf.Status.ForwardStatuses, newStatuses) {
		// the forwards didn't actually change, so skip the update
		return nil
	}

	update := pf.DeepCopy()
	update.Status.ForwardStatuses = newStatuses
	return client.IgnoreNotFound(r.ctrlClient.Status().Update(ctx, update))
}

func (r *Reconciler) onePortForward(ctx context.Context, entry *portForwardEntry, forward Forward) {
	logError := func(err error) {
		logger.Get(ctx).Infof("Reconnecting... Error port-forwarding %s (%d -> %d): %v",
			entry.meta.Annotations[v1alpha1.AnnotationManifest],
			forward.LocalPort, forward.ContainerPort, err)
	}

	pf, err := entry.client.CreatePortForwarder(
		ctx,
		k8s.Namespace(entry.spec.Namespace),
		k8s.PodID(entry.spec.PodName),
		int(forward.LocalPort),
		int(forward.ContainerPort),
		forward.Host)
	if err != nil {
		logError(err)
		entry.setStatus(forward, ForwardStatus{
			LocalPort:     forward.LocalPort,
			ContainerPort: forward.ContainerPort,
			Error:         err.Error(),
		})
		r.requeuer.Add(entry.name)
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
		case <-ctx.Done():
			// context canceled before forward was every ready
			return
		case <-doneCh:
			// forward initialization errored at start before ready
			return
		case <-readyCh:
			entry.setStatus(forward, ForwardStatus{
				LocalPort:     int32(pf.LocalPort()),
				ContainerPort: forward.ContainerPort,
				Addresses:     pf.Addresses(),
				StartedAt:     apis.NowMicro(),
			})
			r.requeuer.Add(entry.name)
		}
	}()

	err = pf.ForwardPorts()
	close(doneCh)
	if err != nil {
		logError(err)
		entry.setStatus(forward, ForwardStatus{
			LocalPort:     int32(pf.LocalPort()),
			ContainerPort: forward.ContainerPort,
			Addresses:     pf.Addresses(),
			Error:         err.Error(),
		})
		r.requeuer.Add(entry.name)
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
	name   types.NamespacedName
	meta   metav1.ObjectMeta
	spec   v1alpha1.PortForwardSpec
	ctx    context.Context
	cancel func()

	mu     sync.Mutex
	status map[Forward]ForwardStatus
	client k8s.Client
}

func newEntry(ctx context.Context, pf *PortForward, cli k8s.Client) *portForwardEntry {
	ctx, cancel := context.WithCancel(ctx)
	return &portForwardEntry{
		name:   types.NamespacedName{Name: pf.Name, Namespace: pf.Namespace},
		meta:   pf.ObjectMeta,
		spec:   pf.Spec,
		ctx:    ctx,
		cancel: cancel,
		status: make(map[Forward]ForwardStatus),
		client: cli,
	}
}

func (e *portForwardEntry) setStatus(spec Forward, status ForwardStatus) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.status[spec] = status
}

func (e *portForwardEntry) statuses() []ForwardStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	var statuses []ForwardStatus
	for _, s := range e.status {
		statuses = append(statuses, *s.DeepCopy())
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

func indexPortForward(obj ctrlclient.Object) []indexer.Key {
	var keys []indexer.Key
	pf := obj.(*v1alpha1.PortForward)

	if pf.Spec.Cluster != "" {
		keys = append(keys, indexer.Key{
			Name: clusterNN(pf),
			GVK:  clusterGVK,
		})
	}

	return keys
}

func clusterNN(pf *v1alpha1.PortForward) types.NamespacedName {
	return types.NamespacedName{
		Namespace: pf.ObjectMeta.Namespace,
		Name:      pf.Spec.Cluster,
	}
}
