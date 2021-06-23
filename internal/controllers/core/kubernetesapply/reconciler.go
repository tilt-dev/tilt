package kubernetesapply

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
)

type Reconciler struct {
	kCli       k8s.Client
	ctrlClient ctrlclient.Client
	indexer    *indexer.Indexer
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesApply{}).
		Watches(&source.Kind{Type: &v1alpha1.ImageMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, kCli k8s.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		kCli:       kCli,
		ctrlClient: ctrlClient,
		indexer:    indexer.NewIndexer(scheme, indexImageMap),
	}
}

// Reconcile manages namespace watches for the modified KubernetesApply object.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var ka v1alpha1.KubernetesApply
	err := r.ctrlClient.Get(ctx, request.NamespacedName, &ka)
	r.indexer.OnReconcile(request.NamespacedName, &ka)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !ka.ObjectMeta.DeletionTimestamp.IsZero() {
		// delete
		return ctrl.Result{}, nil
	}

	// add or replace

	return ctrl.Result{}, nil
}

var imGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

// Find all the objects we need to watch based on the Cmd model.
func indexImageMap(obj client.Object) []indexer.Key {
	ka := obj.(*v1alpha1.KubernetesApply)
	result := []indexer.Key{}

	for _, name := range ka.Spec.ImageMaps {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: name},
			GVK:  imGVK,
		})
	}
	return result
}
