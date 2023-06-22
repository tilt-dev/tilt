package dockerimage

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dockerimages"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var clusterGVK = v1alpha1.SchemeGroupVersion.WithKind("Cluster")

// Manages the DockerImage API object.
type Reconciler struct {
	client   ctrlclient.Client
	st       store.RStore
	indexer  *indexer.Indexer
	docker   docker.Client
	ib       *build.ImageBuilder
	requeuer *indexer.Requeuer

	mu      sync.Mutex
	results map[types.NamespacedName]*result
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, st store.RStore, scheme *runtime.Scheme, docker docker.Client, ib *build.ImageBuilder) *Reconciler {
	return &Reconciler{
		client:   client,
		st:       st,
		indexer:  indexer.NewIndexer(scheme, indexDockerImage),
		docker:   docker,
		ib:       ib,
		results:  make(map[types.NamespacedName]*result),
		requeuer: indexer.NewRequeuer(),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	nn := req.NamespacedName
	obj := &v1alpha1.DockerImage{}
	err := r.client.Get(ctx, nn, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	r.indexer.OnReconcile(nn, obj)

	if apierrors.IsNotFound(err) || obj.ObjectMeta.DeletionTimestamp != nil {
		delete(r.results, nn)
		r.st.Dispatch(dockerimages.NewDockerImageDeleteAction(nn.Name))
		return ctrl.Result{}, nil
	}

	r.st.Dispatch(dockerimages.NewDockerImageUpsertAction(obj))

	err = r.maybeUpdateImageStatus(ctx, nn, obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.maybeUpdateImageMapStatus(ctx, nn)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Build the image, and push it if necessary.
//
// The error is simply the "main" build failure reason.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	iTarget model.ImageTarget,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	ps *build.PipelineState) (store.ImageBuildResult, error) {

	// TODO(nick): It might make sense to reset the ImageMapStatus here
	// to an empty image while the image is building. maybe?
	// I guess it depends on how image reconciliation works, and
	// if you want the live container to keep receiving updates
	// while an image build is going on in parallel.
	startTime := apis.NowMicro()
	nn := types.NamespacedName{Name: iTarget.DockerImageName}
	r.setImageStatus(nn, ToBuildingStatus(iTarget, startTime))

	// Requeue the reconciler twice: once when the build has started and once
	// after it has finished.
	r.requeuer.Add(nn)
	defer r.requeuer.Add(nn)

	refs, stages, err := r.ib.Build(ctx, iTarget, nil, cluster, imageMaps, ps)
	if err != nil {
		r.setImageStatus(nn, ToCompletedFailStatus(iTarget, startTime, stages, err))
		return store.ImageBuildResult{}, err
	}

	r.setImageStatus(nn, ToCompletedSuccessStatus(iTarget, startTime, stages, refs))

	buildResult, err := UpdateImageMap(
		ctx, r.docker,
		iTarget, cluster, imageMaps, &startTime, refs)
	if err != nil {
		return store.ImageBuildResult{}, err
	}
	r.setImageMapStatus(nn, iTarget, buildResult.ImageMapStatus)
	return buildResult, nil
}

func (r *Reconciler) ensureResult(nn types.NamespacedName) *result {
	res, ok := r.results[nn]
	if !ok {
		res = &result{}
		r.results[nn] = res
	}
	return res
}

func (r *Reconciler) setImageStatus(nn types.NamespacedName, status v1alpha1.DockerImageStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResult(nn)
	result.image = status
}

func (r *Reconciler) setImageMapStatus(nn types.NamespacedName, iTarget model.ImageTarget, status v1alpha1.ImageMapStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResult(nn)
	result.imageMapName = iTarget.ImageMapName()
	result.imageMap = status
}

// Update the DockerImage status if necessary.
func (r *Reconciler) maybeUpdateImageStatus(ctx context.Context, nn types.NamespacedName, obj *v1alpha1.DockerImage) error {
	newStatus := v1alpha1.DockerImageStatus{}
	existing, ok := r.results[nn]
	if ok {
		newStatus = existing.image
	}

	if apicmp.DeepEqual(obj.Status, newStatus) {
		return nil
	}

	update := obj.DeepCopy()
	update.Status = *(newStatus.DeepCopy())

	return r.client.Status().Update(ctx, update)
}

// Update the ImageMap status if necessary.
func (r *Reconciler) maybeUpdateImageMapStatus(ctx context.Context, nn types.NamespacedName) error {

	existing, ok := r.results[nn]
	if !ok || existing.imageMapName == "" {
		return nil
	}

	var obj v1alpha1.ImageMap
	imNN := types.NamespacedName{Name: existing.imageMapName}
	err := r.client.Get(ctx, imNN, &obj)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	newStatus := existing.imageMap
	if apicmp.DeepEqual(obj.Status, newStatus) {
		return nil
	}

	update := obj.DeepCopy()
	update.Status = *(newStatus.DeepCopy())

	return r.client.Status().Update(ctx, update)
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DockerImage{}).
		WatchesRawSource(r.requeuer, handler.Funcs{}).
		Watches(&v1alpha1.Cluster{},
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

type result struct {
	image        v1alpha1.DockerImageStatus
	imageMapName string
	imageMap     v1alpha1.ImageMapStatus
}
