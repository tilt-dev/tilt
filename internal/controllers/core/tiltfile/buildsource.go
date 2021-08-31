package tiltfile

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

// BuildEntry is vestigial, but currently used to help manage state about a tiltfile build.
type BuildEntry struct {
	Name                  model.ManifestName
	FilesChanged          []string
	BuildReason           model.BuildReason
	UserConfigState       model.UserConfigState
	TiltfilePath          string
	CheckpointAtExecStart logstore.Checkpoint
	LoadCount             int
}

func (be *BuildEntry) WithLogger(ctx context.Context, st store.RStore) context.Context {
	actionWriter := NewTiltfileLogWriter(be.Name, st, be.LoadCount)
	return logger.CtxWithLogHandler(ctx, actionWriter)
}

// BuildSource is vestigial, but currently used to help re-run the reconciler.
type BuildSource struct {
	mu sync.Mutex
	q  workqueue.RateLimitingInterface
}

var _ source.Source = &BuildSource{}

func NewBuildSource() *BuildSource {
	return &BuildSource{}
}

func (s *BuildSource) Start(ctx context.Context, handler handler.EventHandler, q workqueue.RateLimitingInterface, ps ...predicate.Predicate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.q = q
	return nil
}

func (s *BuildSource) Add(nn types.NamespacedName) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.q != nil {
		s.q.Add(reconcile.Request{
			NamespacedName: nn,
		})
	}
}
