package podlogstream

import (
	"context"
	"sync"

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
}

var _ source.Source = &PodSource{}

func NewPodSource(ctx context.Context, kClient k8s.Client, scheme *runtime.Scheme) *PodSource {
	return &PodSource{
		ctx:                ctx,
		indexer:            indexer.NewIndexer(scheme, indexPodLogStream),
		kClient:            kClient,
		watchesByNamespace: make(map[string]podWatch),
	}
}

func (s *PodSource) Start(ctx context.Context, handler handler.EventHandler, q workqueue.RateLimitingInterface, ps ...predicate.Predicate) error {
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
func (s *PodSource) handleReconcileRequest(ctx context.Context, name types.NamespacedName, pls *PodLogStream) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.indexer.OnReconcile(name, pls)

	ns := pls.Spec.Namespace
	_, ok := s.watchesByNamespace[ns]
	if !ok {
		ctx, cancel := context.WithCancel(ctx)
		pw := podWatch{ctx: ctx, cancel: cancel, namespace: ns}
		s.watchesByNamespace[ns] = pw
		go s.doWatch(pw)
	}
}

// Process pod events and make sure they trigger a reconcile.
func (s *PodSource) doWatch(pw podWatch) {
	defer pw.cancel()

	podCh, err := s.kClient.WatchPods(s.ctx, k8s.Namespace(pw.namespace))
	if err != nil {
		logger.Get(pw.ctx).Errorf("watching pods: %v", err)
		return
	}

	for {
		select {
		case <-pw.ctx.Done():
			return

		case pod, ok := <-podCh:
			if !ok {
				return
			}
			s.handlePod(pod)
			continue
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

// Find all the objects we need to watch based on the PodLogStream
func indexPodLogStream(obj client.Object) []indexer.Key {
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
