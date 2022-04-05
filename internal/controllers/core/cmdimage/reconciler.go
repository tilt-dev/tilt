package cmdimage

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockerimage"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Manages the CmdImage API object.
type Reconciler struct {
	client ctrlclient.Client
	docker docker.Client
	ib     *build.ImageBuilder
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, docker docker.Client, ib *build.ImageBuilder) *Reconciler {
	return &Reconciler{
		client: client,
		docker: docker,
		ib:     ib,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &v1alpha1.CmdImage{}
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || obj.ObjectMeta.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
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
	MaybeUpdateStatus(ctx, r.client, iTarget, ToBuildingStatus(iTarget, startTime))

	refs, _, err := r.ib.Build(ctx, iTarget, cluster, imageMaps, ps)
	if err != nil {
		MaybeUpdateStatus(ctx, r.client, iTarget, ToCompletedFailStatus(iTarget, startTime, err))
		return store.ImageBuildResult{}, err
	}

	MaybeUpdateStatus(ctx, r.client, iTarget, ToCompletedSuccessStatus(iTarget, startTime, refs))

	return dockerimage.UpdateImageMap(
		ctx, r.client, r.docker,
		iTarget, cluster, imageMaps, &startTime, refs)
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CmdImage{})

	return b, nil
}
