package uiresource

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Creates UIResource objects from the EngineState
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

		logger.Get(ctx).Infof("listing uiresource: %v", err)
		return nil
	}

	storedMap := make(map[types.NamespacedName]v1alpha1.UIResource)
	toReconcile := make(map[types.NamespacedName]*v1alpha1.UIResource, len(currentResources))
	for _, r := range storedList.Items {
		storedMap[types.NamespacedName{Name: r.Name}] = r
		toReconcile[types.NamespacedName{Name: r.Name}] = nil
	}
	for _, r := range currentResources {
		toReconcile[types.NamespacedName{Name: r.Name}] = r
	}

	for name, resource := range toReconcile {
		if resource == nil {
			// If there's no current version of this resource, we should delete it.
			err := s.client.Delete(ctx, &v1alpha1.UIResource{ObjectMeta: metav1.ObjectMeta{Name: name.Name}})
			if err != nil && !apierrors.IsNotFound(err) {
				st.Dispatch(store.NewErrorAction(fmt.Errorf("deleting resource %s: %v", name.Name, err)))
				return nil
			}
			continue
		}

		stored, isStored := storedMap[name]
		if !isStored {
			// If there's a current version but nothing stored,
			// create it.
			err := s.client.Create(ctx, resource)
			if err != nil {
				logger.Get(ctx).Infof("creating uiresource %s: %v", name.Name, err)
				return nil
			}
			continue
		}

		if !apicmp.DeepEqual(resource.Status, stored.Status) {
			// If the current version is different than what's stored, update it.
			update := &v1alpha1.UIResource{
				ObjectMeta: *stored.ObjectMeta.DeepCopy(),
				Spec:       *stored.Spec.DeepCopy(),
				Status:     *resource.Status.DeepCopy(),
			}
			update.ObjectMeta.SetLabels(resource.GetObjectMeta().Labels) // (lizz) it sounds like we want to do this upstream, within an `ObjectMeta` field we add to the manifest
			err = s.client.Status().Update(ctx, update)
			if err != nil {
				logger.Get(ctx).Infof("updating uiresource %s: %v", name.Name, err)
				return nil
			}
			continue
		}
	}

	return nil
}

var _ store.Subscriber = &Subscriber{}
