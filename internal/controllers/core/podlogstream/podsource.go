package podlogstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var podGVK = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
var nsGVK = schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}

// Helper struct that captures Pod changes and queues up a Reconcile()
// call for any PodLogStream watching that pod.
type PodSource struct {
	ctx     context.Context
	indexer *indexer.Indexer
	kClient k8s.Client
	handler handler.EventHandler
	q       workqueue.RateLimitingInterface
	clock   clockwork.Clock

	watchesByNamespace map[string]*podWatch
	mu                 sync.Mutex
}

type podWatch struct {
	ctx       context.Context
	cancel    func()
	namespace string

	// Only populated if ctx.Err() != nil (the context has been cancelled)
	finishedAt time.Time
	error      error
}

var _ source.Source = &PodSource{}

func NewPodSource(ctx context.Context, kClient k8s.Client, scheme *runtime.Scheme, clock clockwork.Clock) *PodSource {
	return &PodSource{
		ctx:                ctx,
		indexer:            indexer.NewIndexer(scheme, indexPodLogStreamForKubernetes),
		kClient:            kClient,
		watchesByNamespace: make(map[string]*podWatch),
		clock:              clock,
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
func (s *PodSource) handleReconcileRequest(ctx context.Context, name types.NamespacedName, pls *PodLogStream) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.indexer.OnReconcile(name, pls)

	var err error
	ns := pls.Spec.Namespace
	if ns != "" {
		pw, ok := s.watchesByNamespace[ns]
		if !ok {
			ctx, cancel := context.WithCancel(ctx)
			pw = &podWatch{ctx: ctx, cancel: cancel, namespace: ns}
			s.watchesByNamespace[ns] = pw
			go s.doWatch(pw)
		}

		if pw.ctx.Err() != nil {
			err = pw.ctx.Err()
			if pw.error != nil {
				err = pw.error
			}
		}
	}
	return err
}

// Process pod events and make sure they trigger a reconcile.
func (s *PodSource) doWatch(pw *podWatch) {
	defer func() {
		// If the watch wasn't cancelled and there's no other error,
		// record a generic error.
		if pw.error == nil && pw.ctx.Err() == nil {
			pw.error = fmt.Errorf("watch disconnected")
		}

		pw.finishedAt = s.clock.Now()
		pw.cancel()
		s.requeueIndexerKey(indexer.Key{Name: types.NamespacedName{Name: pw.namespace}, GVK: nsGVK})
	}()

	pw.finishedAt = time.Time{}
	pw.error = nil

	podCh, err := s.kClient.WatchPods(s.ctx, k8s.Namespace(pw.namespace))
	if err != nil {
		pw.error = fmt.Errorf("watching pods: %v", err)
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

	s.requeueIndexerKey(indexer.Key{Name: podNN, GVK: podGVK})
}

func (s *PodSource) requeueIndexerKey(key indexer.Key) {
	s.mu.Lock()
	requests := s.indexer.EnqueueKey(key)
	q := s.q
	s.mu.Unlock()

	if q == nil {
		return
	}

	for _, req := range requests {
		q.Add(req)
	}
}
func (s *PodSource) requeueStream(name types.NamespacedName) {
	s.mu.Lock()
	q := s.q
	s.mu.Unlock()

	if q == nil {
		return
	}
	q.Add(reconcile.Request{NamespacedName: name})
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
	return []indexer.Key{
		// Watch events broadcast on the whole namespace.
		indexer.Key{
			Name: types.NamespacedName{Name: pls.Spec.Namespace},
			GVK:  nsGVK,
		},
		// Watch events on this specific Pod.
		indexer.Key{
			Name: types.NamespacedName{Name: pls.Spec.Pod, Namespace: pls.Spec.Namespace},
			GVK:  podGVK,
		},
	}
}
