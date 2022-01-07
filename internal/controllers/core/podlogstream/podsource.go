package podlogstream

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const maxRestartBackoff = 5 * time.Minute

var podGVK = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}

// Helper struct that captures Pod changes and queues up a Reconcile()
// call for any PodLogStream watching that pod.
type PodSource struct {
	ctx     context.Context
	indexer *indexer.Indexer
	kClient k8s.Client
	handler handler.EventHandler
	q       workqueue.RateLimitingInterface
	mu      sync.Mutex

	watchesByNamespace map[string]podWatch
}

type podWatch struct {
	ctx       context.Context
	cancel    func()
	namespace string
	watchers  map[types.NamespacedName]bool
}

var _ source.Source = &PodSource{}

func NewPodSource(ctx context.Context, kClient k8s.Client, scheme *runtime.Scheme) *PodSource {
	return &PodSource{
		ctx:                ctx,
		indexer:            indexer.NewIndexer(scheme, indexPodLogStreamForKubernetes),
		kClient:            kClient,
		watchesByNamespace: make(map[string]podWatch),
	}
}

// Start initializes the PodSource.
//
// NOTE: We ignore the context here because it's managed by controller-runtime and will not have Tilt values (e.g. logger).
// 	See https://github.com/kubernetes-sigs/controller-runtime/issues/1752.
func (s *PodSource) Start(_ context.Context, handler handler.EventHandler, q workqueue.RateLimitingInterface, _ ...predicate.Predicate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.q = q
	s.handler = handler
	return nil
}

func (s *PodSource) TearDown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, pw := range s.watchesByNamespace {
		pw.cancel()
		delete(s.watchesByNamespace, k)
	}
}

// Register the pods for this stream.
//
// Set up any watches we need.
func (s *PodSource) handleReconcileRequest(plsNN types.NamespacedName, pls *PodLogStream) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.indexer.OnReconcile(plsNN, pls)

	var ns string
	if pls != nil {
		ns = pls.Spec.Namespace
	}

	// ensure namespace watch registered if PLS object exists + has valid spec
	if ns != "" {
		pw, ok := s.watchesByNamespace[ns]
		if !ok {
			pw = newPodWatch(s.ctx, ns)
			s.watchesByNamespace[ns] = pw
			go s.doWatchWithRetry(pw)
		}
		pw.watchers[plsNN] = true
	}

	// clean up any pre-existing namespace watches
	// this covers both PLS changing namespace in spec + PLS deletion
	for nsKey, watch := range s.watchesByNamespace {
		if watch.watchers[plsNN] && nsKey != ns {
			delete(watch.watchers, plsNN)
			s.maybeCleanupWatch(nsKey)
		}
	}
}

// maybeCleanupWatch removes a namespace watch if there are no more active watchers.
func (s *PodSource) maybeCleanupWatch(nsKey string) {
	watch, ok := s.watchesByNamespace[nsKey]
	if !ok {
		return
	}

	if len(watch.watchers) != 0 {
		// there's still active watchers
		return
	}

	watch.cancel()
	delete(s.watchesByNamespace, nsKey)
}

func (s *PodSource) doWatchWithRetry(pw podWatch) {
	defer pw.cancel()

	restartBackoff := 500 * time.Millisecond
	var lastErrMsg string
	for {
		err := s.doWatch(pw)
		if err != nil {
			if err.Error() != lastErrMsg {
				logger.Get(pw.ctx).Errorf("watching pods: %v", err)
			}
			lastErrMsg = err.Error()
		} else {
			lastErrMsg = ""
		}

		restartBackoff = restartBackoff * 2
		if restartBackoff > maxRestartBackoff {
			restartBackoff = maxRestartBackoff
		}

		select {
		case <-pw.ctx.Done():
			return
		case <-time.After(restartBackoff):
			// retry
		}
	}
}

// Process pod events and make sure they trigger a reconcile.
func (s *PodSource) doWatch(pw podWatch) error {
	podCh, err := s.kClient.WatchPods(pw.ctx, k8s.Namespace(pw.namespace))
	if err != nil {
		return err
	}

	for {
		select {
		case <-pw.ctx.Done():
			return nil

		case pod, ok := <-podCh:
			if !ok {
				return nil
			}
			s.handlePod(pod)
		}
	}
}

// Turn all pod events into Reconcile() calls.
func (s *PodSource) handlePod(obj k8s.ObjectUpdate) {
	podNN, ok := obj.AsNamespacedName()
	if !ok {
		return
	}

	s.mu.Lock()
	requests := s.indexer.EnqueueKey(indexer.Key{
		Name: podNN,
		GVK:  podGVK,
	})
	q := s.q
	s.mu.Unlock()

	if q == nil {
		return
	}

	for _, req := range requests {
		q.Add(req)
	}
}

func newPodWatch(ctx context.Context, ns string) podWatch {
	ctx, cancel := context.WithCancel(ctx)
	return podWatch{
		ctx:       ctx,
		cancel:    cancel,
		namespace: ns,
		watchers:  make(map[types.NamespacedName]bool),
	}
}

// indexPodLogStreamForKubernetes indexes a PodLogStream object and returns keys
// for Pods from the K8s cluster that it watches.
//
// See also: indexPodLogStreamForTiltAPI which indexes a PodLogStream object
// and returns keys for objects from the Tilt apiserver that it watches.
func indexPodLogStreamForKubernetes(obj client.Object) []indexer.Key {
	pls := obj.(*v1alpha1.PodLogStream)
	if pls.Spec.Pod == "" {
		return nil
	}
	podNN := types.NamespacedName{Name: pls.Spec.Pod, Namespace: pls.Spec.Namespace}
	return []indexer.Key{
		indexer.Key{
			Name: podNN,
			GVK:  podGVK,
		},
	}
}
