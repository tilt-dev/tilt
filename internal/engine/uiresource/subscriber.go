package uiresource

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Creates UIResource objects from the EngineState
type Subscriber struct {
	lastResources map[types.NamespacedName]*v1alpha1.UIResource
}

func NewSubscriber() *Subscriber {
	return &Subscriber{
		lastResources: make(map[types.NamespacedName]*v1alpha1.UIResource),
	}
}

func (s *Subscriber) currentResources(store store.RStore) ([]*v1alpha1.UIResource, error) {
	state := store.RLockState()
	defer store.RUnlockState()
	return webview.ToUIResourceList(state)
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	currentResources, err := s.currentResources(st)
	if err != nil {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("cannot convert UIResource: %v", err)))
		return
	}

	// Collect a list of all the resources to reconcile and their most recent version.
	toReconcile := make(map[types.NamespacedName]*v1alpha1.UIResource, len(currentResources))
	for _, r := range s.lastResources {
		toReconcile[types.NamespacedName{Name: r.Name}] = nil
	}
	for _, r := range currentResources {
		toReconcile[types.NamespacedName{Name: r.Name}] = r
	}

	for name, current := range toReconcile {
		if current == nil {
			// If there's no current version of this resource, we should delete it.
			st.Dispatch(NewUIResourceDeleteAction(name))
			delete(s.lastResources, name)
			continue
		}

		last, exists := s.lastResources[name]
		if !exists {
			// If there's a current version but no last version of this resource,
			// create it.
			st.Dispatch(NewUIResourceCreateAction(current))
			s.lastResources[name] = current
			continue
		}

		if !equality.Semantic.DeepEqual(last.Status, current.Status) {
			// If the current version is different than the last version, update it.
			st.Dispatch(NewUIResourceUpdateStatusAction(current))
			s.lastResources[name] = current
		}
	}
}

var _ store.Subscriber = &Subscriber{}
