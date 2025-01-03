package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Small helper class for triggering a Reconcile() from a goroutine.
type Requeuer struct {
	mu sync.Mutex
	q  workqueue.TypedRateLimitingInterface[reconcile.Request]
}

var _ source.Source = &Requeuer{}
var _ fmt.Stringer = &Requeuer{}

func NewRequeuer() *Requeuer {
	return &Requeuer{}
}

func (s *Requeuer) String() string {
	return "requeuer"
}

func (s *Requeuer) Start(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
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

type RequeueForTestResult struct {
	ObjName types.NamespacedName
	Error   error
	Result  reconcile.Result
}

func StartSourceForTesting(
	ctx context.Context,
	s source.Source,
	reconciler reconcile.Reconciler,
	resultChan chan RequeueForTestResult,
) {
	q := workqueue.NewTypedRateLimitingQueue[reconcile.Request](
		workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
			time.Millisecond, time.Millisecond))
	_ = s.Start(ctx, q)

	go func() {
		for ctx.Err() == nil {
			req, shutdown := q.Get()
			if shutdown {
				return
			}

			res, err := reconciler.Reconcile(ctx, req)
			if resultChan != nil {
				result := RequeueForTestResult{
					ObjName: req.NamespacedName,
					Error:   err,
					Result:  res,
				}
				resultChan <- result
			}

			q.Done(req)
		}
	}()
}
