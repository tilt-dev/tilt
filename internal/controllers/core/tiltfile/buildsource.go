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
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

// BuildSource is a way for legacy Tilt reconcilers to
// notify the reconciliation loop that a Tiltfile needs to be rebuilt.
//
// We model this as a controller-runtime Source, which is intended for watching
// external resources that trigger reconciliation.

type BuildEntry struct {
	Name                  model.ManifestName
	FilesChanged          []string
	BuildReason           model.BuildReason
	UserConfigState       model.UserConfigState
	TiltfilePath          string
	CheckpointAtExecStart logstore.Checkpoint
	EngineMode            store.EngineMode
}

type BuildSource struct {
	mu    sync.Mutex
	entry *BuildEntry
	q     workqueue.RateLimitingInterface
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

func (s *BuildSource) Entry() *BuildEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entry
}

func (s *BuildSource) SetEntry(e *BuildEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry = e
	if e != nil && s.q != nil {
		s.q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: e.Name.String()},
		})
	}
}
