package liveupdate

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var discoveryGVK = v1alpha1.SchemeGroupVersion.WithKind("KubernetesDiscovery")
var applyGVK = v1alpha1.SchemeGroupVersion.WithKind("KubernetesApply")

// Manages the LiveUpdate API object.
type Reconciler struct {
	client  ctrlclient.Client
	indexer *indexer.Indexer
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		client:  client,
		indexer: indexer.NewIndexer(scheme, indexLiveUpdate),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lu := &v1alpha1.LiveUpdate{}
	err := r.client.Get(ctx, req.NamespacedName, lu)
	r.indexer.OnReconcile(req.NamespacedName, lu)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("liveupdate reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || lu.ObjectMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LiveUpdate{}).
		Watches(&source.Kind{Type: &v1alpha1.KubernetesDiscovery{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.KubernetesApply{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

// indexLiveUpdate returns keys of objects referenced _by_ the LiveUpdate object for reverse lookup including:
// 	- KubernetesDiscovery
//	- KubernetesApply
func indexLiveUpdate(obj ctrlclient.Object) []indexer.Key {
	lu := obj.(*v1alpha1.LiveUpdate)
	var result []indexer.Key
	if lu.Spec.Selector.Kubernetes != nil {
		if lu.Spec.Selector.Kubernetes.DiscoveryName != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      lu.Spec.Selector.Kubernetes.DiscoveryName,
				},
				GVK: discoveryGVK,
			})
		}

		if lu.Spec.Selector.Kubernetes.ApplyName != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      lu.Spec.Selector.Kubernetes.ApplyName,
				},
				GVK: applyGVK,
			})
		}
	}
	return result
}
