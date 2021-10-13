package uiresource

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// UIResource objects are created/deleted by the Tiltfile controller.
//
// This subscriber only updates their status.
type Subscriber struct {
	client ctrlclient.Client
}

func NewSubscriber(client ctrlclient.Client) *Subscriber {
	return &Subscriber{
		client: client,
	}
}

func (s *Subscriber) currentResources(store store.RStore) ([]*v1alpha1.UIResource, error) {
	state := store.RLockState()
	defer store.RUnlockState()
	return webview.ToUIResourceList(state)
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	currentResources, err := s.currentResources(st)
	if err != nil {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("cannot convert UIResource: %v", err)))
		return nil
	}

	// Collect a list of all the resources to reconcile and their most recent version.
	storedList := &v1alpha1.UIResourceList{}
	err = s.client.List(ctx, storedList)

	if err != nil {
		// If the cache hasn't started yet, that's OK.
		// We'll get it on the next OnChange()
		if _, ok := err.(*cache.ErrCacheNotStarted); ok {
			return nil
		}

		return err
	}

	storedMap := make(map[string]v1alpha1.UIResource)
	for _, r := range storedList.Items {
		storedMap[r.Name] = r
	}

	for _, r := range currentResources {
		stored, isStored := storedMap[r.Name]
		if !isStored {
			continue
		}

		// DisableStatus counts are managed by the UIResource reconciler rather than calculated from
		// EngineState, so leave their values in place
		r.Status.DisableStatus.DisabledCount = stored.Status.DisableStatus.DisabledCount
		r.Status.DisableStatus.EnabledCount = stored.Status.DisableStatus.EnabledCount

		if !apicmp.DeepEqual(r.Status, stored.Status) {
			// If the current version is different than what's stored, update it.
			update := stored.DeepCopy()
			update.Status = r.Status
			err = s.client.Status().Update(ctx, update)
			if err != nil {
				return err
			}
			continue
		}
	}

	return nil
}

var _ store.Subscriber = &Subscriber{}
