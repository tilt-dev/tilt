package dockercomposeservice

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Reconciler struct {
	st         store.RStore
	ctrlClient ctrlclient.Client
	indexer    *indexer.Indexer
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DockerComposeService{}).
		Watches(&source.Kind{Type: &v1alpha1.ImageMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, st store.RStore, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		ctrlClient: ctrlClient,
		indexer:    indexer.NewIndexer(scheme, indexDockerComposeService),
		st:         st,
	}
}

// Reconcile manages namespace watches for the modified DockerComposeService object.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var obj v1alpha1.DockerComposeService
	err := r.ctrlClient.Get(ctx, nn, &obj)
	r.indexer.OnReconcile(nn, &obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// Apply the DockerCompose service spec, unconditionally.
//
// Update the apiserver when finished.
//
// We expose this as a public method as a hack! Currently, in Tilt, BuildController
// handles dependencies between resources. The API server doesn't know about build
// dependencies yet. So Tiltfile-owned resources are applied manually, rather than
// going through the normal reconcile system.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) (v1alpha1.DockerComposeServiceStatus, error) {
	// TK(nick): Fill this out.
	return v1alpha1.DockerComposeServiceStatus{}, nil
}

var imGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

// indexDockerComposeService returns keys for all the objects we need to watch based on the spec.
func indexDockerComposeService(obj client.Object) []indexer.Key {
	dcs := obj.(*v1alpha1.DockerComposeService)
	result := []indexer.Key{}
	for _, name := range dcs.Spec.ImageMaps {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: name},
			GVK:  imGVK,
		})
	}

	return result
}
