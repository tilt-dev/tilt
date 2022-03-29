package dockerimage

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
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

var clusterGVK = v1alpha1.SchemeGroupVersion.WithKind("Cluster")

// Manages the DockerImage API object.
type Reconciler struct {
	client  ctrlclient.Client
	indexer *indexer.Indexer
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		client:  client,
		indexer: indexer.NewIndexer(scheme, indexDockerImage),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &v1alpha1.DockerImage{}
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	r.indexer.OnReconcile(req.NamespacedName, obj)

	if apierrors.IsNotFound(err) || obj.ObjectMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DockerImage{}).
		Watches(&source.Kind{Type: &v1alpha1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func indexDockerImage(obj ctrlclient.Object) []indexer.Key {
	var keys []indexer.Key

	di := obj.(*v1alpha1.DockerImage)
	if di != nil && di.Spec.Cluster != "" {
		keys = append(keys, indexer.Key{
			Name: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      di.Spec.Cluster,
			},
			GVK: clusterGVK,
		})
	}

	return keys
}
