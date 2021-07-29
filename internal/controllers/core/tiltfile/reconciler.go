package tiltfile

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
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Reconciler struct {
	st           store.RStore
	tfl          tiltfile.TiltfileLoader
	dockerClient docker.Client
	ctrlClient   ctrlclient.Client
	indexer      *indexer.Indexer
	buildSource  *BuildSource
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Tiltfile{}).
		Watches(&source.Kind{Type: &v1alpha1.FileWatch{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(r.buildSource, handler.Funcs{})

	return b, nil
}

func NewReconciler(st store.RStore, tfl tiltfile.TiltfileLoader, dockerClient docker.Client, ctrlClient ctrlclient.Client, scheme *runtime.Scheme, buildSource *BuildSource) *Reconciler {
	return &Reconciler{
		st:           st,
		tfl:          tfl,
		dockerClient: dockerClient,
		ctrlClient:   ctrlClient,
		indexer:      indexer.NewIndexer(scheme, indexTiltfile),
		buildSource:  buildSource,
	}
}

// Reconcile manages Tiltfile execution.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var tf v1alpha1.Tiltfile
	err := r.ctrlClient.Get(ctx, nn, &tf)
	r.indexer.OnReconcile(nn, &tf)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !tf.ObjectMeta.DeletionTimestamp.IsZero() {
		// TODO(nick): Handle deletion
		return ctrl.Result{}, nil
	}

	ctx = store.MustObjectLogHandler(ctx, r.st, &tf)
	_ = ctx

	return ctrl.Result{}, nil
}

// Find all the objects we need to watch based on the tiltfile model.
func indexTiltfile(obj client.Object) []indexer.Key {
	result := []indexer.Key{}
	tiltfile := obj.(*v1alpha1.Tiltfile)
	if tiltfile.Spec.RestartOn != nil {
		fwGVK := v1alpha1.SchemeGroupVersion.WithKind("FileWatch")

		for _, name := range tiltfile.Spec.RestartOn.FileWatches {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: name},
				GVK:  fwGVK,
			})
		}
	}
	return result
}
