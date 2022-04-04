package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmdimage"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockerimage"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesapply"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type ImageBuildAndDeployer struct {
	ib         *ImageBuilder
	analytics  *analytics.TiltAnalytics
	clock      build.Clock
	ctrlClient ctrlclient.Client
	r          *kubernetesapply.Reconciler
}

func NewImageBuildAndDeployer(
	ib *ImageBuilder,
	analytics *analytics.TiltAnalytics,
	c build.Clock,
	ctrlClient ctrlclient.Client,
	r *kubernetesapply.Reconciler,
) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		ib:         ib,
		analytics:  analytics,
		clock:      c,
		ctrlClient: ctrlClient,
		r:          r,
	}
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) != 1 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("ImageBuildAndDeployer does not support these specs")
	}

	kTarget := kTargets[0]

	startTime := time.Now()
	defer func() {
		ibd.analytics.Timer("build.image", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	q, err := NewImageTargetQueue(ctx, iTargets, stateSet, ibd.ib.CanReuseRef)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// each image target has two stages: one for build, and one for push
	numStages := q.CountBuilds()*2 + 1

	reused := q.ReusedResults()
	hasReusedStep := len(reused) > 0
	if hasReusedStep {
		numStages++
	}

	hasDeleteStep := stateSet.FullBuildTriggered()
	if hasDeleteStep {
		numStages++
	}

	ps := build.NewPipelineState(ctx, numStages, ibd.clock)
	defer func() { ps.End(ctx, err) }()

	if hasDeleteStep {
		ps.StartPipelineStep(ctx, "Force update")
		err = ibd.delete(ps.AttachLogger(ctx), kTarget)
		if err != nil {
			return store.BuildResultSet{}, WrapDontFallBackError(err)
		}
		ps.EndPipelineStep(ctx)
	}

	if hasReusedStep {
		ps.StartPipelineStep(ctx, "Loading cached images")
		for _, result := range reused {
			ps.Printf(ctx, "- %s", store.LocalImageRefFromBuildResult(result))
		}
		ps.EndPipelineStep(ctx)
	}

	imageMapSet := make(map[types.NamespacedName]*v1alpha1.ImageMap, len(kTarget.ImageMaps))
	for _, iTarget := range iTargets {
		if iTarget.IsLiveUpdateOnly {
			continue
		}

		var im v1alpha1.ImageMap
		nn := types.NamespacedName{Name: iTarget.ImageMapName()}
		err := ibd.ctrlClient.Get(ctx, nn, &im)
		if err != nil {
			return nil, err
		}
		imageMapSet[nn] = im.DeepCopy()
	}

	err = q.RunBuilds(func(target model.TargetSpec, depResults []store.ImageBuildResult) (store.ImageBuildResult, error) {
		iTarget, ok := target.(model.ImageTarget)
		if !ok {
			return store.ImageBuildResult{}, fmt.Errorf("Not an image target: %T", target)
		}

		// TODO(nick): It might make sense to reset the ImageMapStatus here
		// to an empty image while the image is building. maybe?
		// I guess it depends on how image reconciliation works, and
		// if you want the live container to keep receiving updates
		// while an image build is going on in parallel.
		startTime := apis.NowMicro()
		dockerimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, dockerimage.ToBuildingStatus(iTarget, startTime))
		cmdimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, cmdimage.ToBuildingStatus(iTarget, startTime))
		cluster := stateSet[target.ID()].ClusterOrEmpty()

		refs, stages, err := ibd.ib.Build(ctx, iTarget, cluster, imageMapSet, ps)
		if err != nil {
			dockerimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, dockerimage.ToCompletedFailStatus(iTarget, startTime, stages, err))
			cmdimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, cmdimage.ToCompletedFailStatus(iTarget, startTime, err))
			return store.ImageBuildResult{}, err
		}

		dockerimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, dockerimage.ToCompletedSuccessStatus(iTarget, startTime, stages, refs))
		cmdimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, cmdimage.ToCompletedSuccessStatus(iTarget, startTime, refs))

		result := store.NewImageBuildResult(iTarget.ID(), refs.LocalRef, refs.ClusterRef)
		result.ImageMapStatus.BuildStartTime = &startTime
		nn := types.NamespacedName{Name: iTarget.ImageMapName()}
		im, ok := imageMapSet[nn]
		if !ok {
			return store.ImageBuildResult{}, fmt.Errorf("apiserver missing ImageMap: %s", iTarget.ID().Name)
		}
		im.Status = result.ImageMapStatus
		err = ibd.ctrlClient.Status().Update(ctx, im)
		if err != nil {
			return store.ImageBuildResult{}, fmt.Errorf("updating ImageMap: %v", err)
		}

		return result, nil
	})

	newResults := q.NewResults().ToBuildResultSet()
	if err != nil {
		return newResults, WrapDontFallBackError(err)
	}

	// (If we pass an empty list of refs here (as we will do if only deploying
	// yaml), we just don't inject any image refs into the yaml, nbd.
	k8sResult, err := ibd.deploy(ctx, st, ps, kTarget.ID(), kTarget.KubernetesApplySpec, imageMapSet)
	if err != nil {
		return newResults, WrapDontFallBackError(err)
	}
	newResults[kTarget.ID()] = k8sResult
	return newResults, nil
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(
	ctx context.Context,
	st store.RStore,
	ps *build.PipelineState,
	kTargetID model.TargetID,
	spec v1alpha1.KubernetesApplySpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) (store.K8sBuildResult, error) {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	kTargetNN := types.NamespacedName{Name: kTargetID.Name.String()}
	status := ibd.r.ForceApply(ctx, kTargetNN, spec, imageMaps)
	if status.Error != "" {
		return store.K8sBuildResult{}, fmt.Errorf("%s", status.Error)
	}

	filter, err := k8sconv.NewKubernetesApplyFilter(&status)
	if err != nil {
		return store.K8sBuildResult{}, err
	}
	return store.NewK8sDeployResult(kTargetID, filter), nil
}

// Delete all the resources in the Kubernetes target, to ensure that they restart when
// we re-apply them.
func (ibd *ImageBuildAndDeployer) delete(ctx context.Context, k8sTarget model.K8sTarget) error {
	kTargetNN := types.NamespacedName{Name: k8sTarget.ID().Name.String()}
	return ibd.r.ForceDelete(ctx, kTargetNN, k8sTarget.KubernetesApplySpec, "force update")
}
