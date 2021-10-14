package uiresource

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/uiresources"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Reconciler struct {
	client  ctrlclient.Client
	wsList  *server.WebsocketList
	indexer *indexer.Indexer
	store   store.RStore
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, wsList *server.WebsocketList, store store.RStore, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		client:  client,
		wsList:  wsList,
		store:   store,
		indexer: indexer.NewIndexer(scheme, indexUIResource),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	resource := &v1alpha1.UIResource{}
	err := r.client.Get(ctx, req.NamespacedName, resource)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("uiresource reconcile: %v", err)
	}

	r.indexer.OnReconcile(req.NamespacedName, resource)

	if apierrors.IsNotFound(err) || resource.ObjectMeta.DeletionTimestamp != nil {
		r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
			ws.SendUIResourceUpdate(ctx, req.NamespacedName, nil)
		})

		r.store.Dispatch(uiresources.NewUIResourceDeleteAction(req.Name))
		return ctrl.Result{}, nil
	}

	disableStatus, err := r.disableStatus(ctx, resource)
	if err != nil {
		return ctrl.Result{}, nil
	}

	if !apicmp.DeepEqual(disableStatus, resource.Status.DisableStatus) {
		resource.Status.DisableStatus = disableStatus
		err := r.client.Status().Update(ctx, resource)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.store.Dispatch(uiresources.NewUIResourceUpsertAction(resource))

	r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
		ws.SendUIResourceUpdate(ctx, req.NamespacedName, resource)
	})

	return ctrl.Result{}, nil
}

// we know the list of DisableSources, so just count the statuses of those
// This assumes that the resource doesn't have any objects that set their own, separate DisableSource.
// Long-term, it's probably a more principled solution to watch the api objects belonging to a
// resource and fetch DisableSource/DisableStatus from those.
// For now, this gets us the same result in the api and is dramatically simpler.
func (r *Reconciler) disableStatus(ctx context.Context, resource *v1alpha1.UIResource) (v1alpha1.DisableResourceStatus, error) {
	result := v1alpha1.DisableResourceStatus{
		Sources: resource.Status.DisableStatus.Sources,
	}
	for _, ds := range resource.Status.DisableStatus.Sources {
		isDisabled, _, err := configmap.DisableStatus(ctx, r.client, &ds)
		if err != nil {
			return v1alpha1.DisableResourceStatus{}, nil
		}
		if isDisabled {
			result.DisabledCount += 1
		} else {
			result.EnabledCount += 1
		}
	}
	return result, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.UIResource{}).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func indexUIResource(obj ctrlclient.Object) []indexer.Key {
	var result []indexer.Key
	uir := obj.(*v1alpha1.UIResource)
	cmGVK := v1alpha1.SchemeGroupVersion.WithKind("ConfigMap")

	if uir != nil {
		for _, ds := range uir.Status.DisableStatus.Sources {
			if ds.ConfigMap != nil {
				result = append(result, indexer.Key{
					Name: types.NamespacedName{Name: ds.ConfigMap.Name},
					GVK:  cmGVK,
				})
			}
		}
	}
	return result
}
