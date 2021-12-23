package uiresource

import (
	"context"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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

func (s *Subscriber) currentResources(store store.RStore, disableSources map[string][]v1alpha1.DisableSource) ([]*v1alpha1.UIResource, error) {
	state := store.RLockState()
	defer store.RUnlockState()
	return webview.ToUIResourceList(state, disableSources)
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	// Collect a list of all the resources to reconcile and their most recent version.
	storedList := &v1alpha1.UIResourceList{}
	err := s.client.List(ctx, storedList)

	if err != nil {
		// If the cache hasn't started yet, that's OK.
		// We'll get it on the next OnChange()
		if _, ok := err.(*cache.ErrCacheNotStarted); ok {
			return nil
		}

		return err
	}

	storedMap := make(map[string]v1alpha1.UIResource)
	disableSources := make(map[string][]v1alpha1.DisableSource)
	for _, r := range storedList.Items {
		storedMap[r.Name] = r
		disableSources[r.Name] = r.Status.DisableStatus.Sources
	}

	currentResources, err := s.currentResources(st, disableSources)
	if err != nil {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("cannot convert UIResource: %v", err)))
		return nil
	}

	errs := []error{}
	for _, r := range currentResources {
		stored, isStored := storedMap[r.Name]
		if !isStored {
			continue
		}

		reconcileConditions(r.Status.Conditions, stored.Status.Conditions)

		if !apicmp.DeepEqual(r.Status, stored.Status) {
			// If the current version is different than what's stored, update it.
			update := stored.DeepCopy()
			update.Status = r.Status
			err = s.client.Status().Update(ctx, update)
			if err != nil {
				errs = append(errs, err)
			}
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

// Update the LastTransitionTime against the currently stored conditions.
func reconcileConditions(conds []v1alpha1.UIResourceCondition, stored []v1alpha1.UIResourceCondition) {
	storedMap := make(map[v1alpha1.UIResourceConditionType]v1alpha1.UIResourceCondition, len(stored))
	for _, c := range stored {
		storedMap[c.Type] = c
	}

	for i, c := range conds {
		existing, ok := storedMap[c.Type]
		if !ok {
			continue
		}

		// If the status hasn't changed, fall back to the previous transition time.
		if existing.Status == c.Status {
			c.LastTransitionTime = existing.LastTransitionTime
		}
		conds[i] = c
	}
}

var _ store.Subscriber = &Subscriber{}
