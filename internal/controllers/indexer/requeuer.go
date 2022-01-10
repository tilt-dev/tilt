package indexer

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Small helper class for triggering a Reconcile() from a goroutine.
type Requeuer struct {
	mu sync.Mutex
	q  workqueue.RateLimitingInterface
}

var _ source.Source = &Requeuer{}

func NewRequeuer() *Requeuer {
	return &Requeuer{}
}

func (s *Requeuer) Start(ctx context.Context, handler handler.EventHandler, q workqueue.RateLimitingInterface, ps ...predicate.Predicate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.q = q
	return nil
}

func (s *Requeuer) Add(nn types.NamespacedName) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.q != nil {
		s.q.Add(reconcile.Request{
			NamespacedName: nn,
		})
	}
}

type reconciler interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

func StartRequeuerForTesting(ctx context.Context, requeuer *Requeuer, reconciler reconciler) {
	q := workqueue.NewRateLimitingQueue(
		workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, time.Millisecond))
	_ = requeuer.Start(ctx, handler.Funcs{}, q)

	go func() {
		for ctx.Err() == nil {
			next, shutdown := q.Get()
			if shutdown {
				return
			}

			_, _ = reconciler.Reconcile(ctx, next.(reconcile.Request))
			q.Done(next)
		}
	}()
}
