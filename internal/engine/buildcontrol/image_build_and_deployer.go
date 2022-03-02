package buildcontrol

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmdimage"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockerimage"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesapply"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type KINDLoader interface {
	LoadToKIND(ctx context.Context, ref reference.NamedTagged) error
}

type cmdKINDLoader struct {
	env         k8s.Env
	clusterName k8s.ClusterName
}

func (kl *cmdKINDLoader) LoadToKIND(ctx context.Context, ref reference.NamedTagged) error {
	// In Kind5, --name specifies the name of the cluster in the kubeconfig.
	// In Kind6, the -name parameter is prefixed with 'kind-' before being written to/read from the kubeconfig
	kindName := string(kl.clusterName)
	if kl.env == k8s.EnvKIND6 {
		kindName = strings.TrimPrefix(kindName, "kind-")
	}

	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", ref.String(), "--name", kindName)
	w := logger.NewMutexWriter(logger.Get(ctx).Writer(logger.InfoLvl))
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}

func NewKINDLoader(env k8s.Env, clusterName k8s.ClusterName) KINDLoader {
	return &cmdKINDLoader{
		env:         env,
		clusterName: clusterName,
	}
}

type ImageBuildAndDeployer struct {
	db          *build.DockerBuilder
	ib          *ImageBuilder
	k8sClient   k8s.Client
	env         k8s.Env
	kubeContext k8s.KubeContext
	analytics   *analytics.TiltAnalytics
	clock       build.Clock
	kl          KINDLoader
	ctrlClient  ctrlclient.Client
	r           *kubernetesapply.Reconciler
}

func NewImageBuildAndDeployer(
	db *build.DockerBuilder,
	customBuilder *build.CustomBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	kubeContext k8s.KubeContext,
	analytics *analytics.TiltAnalytics,
	c build.Clock,
	kl KINDLoader,
	ctrlClient ctrlclient.Client,
	r *kubernetesapply.Reconciler,
) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		db:          db,
		ib:          NewImageBuilder(db, customBuilder),
		k8sClient:   k8sClient,
		env:         env,
		kubeContext: kubeContext,
		analytics:   analytics,
		clock:       c,
		kl:          kl,
		ctrlClient:  ctrlClient,
		r:           r,
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

	var cluster v1alpha1.Cluster
	// If the cluster fetch fails, that's OK.
	_ = ibd.ctrlClient.Get(ctx, types.NamespacedName{Name: "default"}, &cluster)

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

		refs, stages, err := ibd.ib.Build(ctx, iTarget, &cluster, imageMapSet, ps)
		if err != nil {
			dockerimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, dockerimage.ToCompletedFailStatus(iTarget, startTime, stages, err))
			cmdimage.MaybeUpdateStatus(ctx, ibd.ctrlClient, iTarget, cmdimage.ToCompletedFailStatus(iTarget, startTime, err))
			return store.ImageBuildResult{}, err
		}

		pushStage := ibd.push(ctx, refs.LocalRef, ps, iTarget, kTarget)
		if pushStage != nil {
			stages = append(stages, *pushStage)
		}

		if pushStage != nil && pushStage.Error != "" {
			err := fmt.Errorf("%s", pushStage.Error)
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

func (ibd *ImageBuildAndDeployer) push(ctx context.Context, ref reference.NamedTagged, ps *build.PipelineState, iTarget model.ImageTarget, kTarget model.K8sTarget) *v1alpha1.DockerImageStageStatus {
	ps.StartPipelineStep(ctx, "Pushing %s", container.FamiliarString(ref))
	defer ps.EndPipelineStep(ctx)

	cbSkip := false
	if iTarget.IsCustomBuild() {
		cbSkip = iTarget.CustomBuildInfo().SkipsPush()
	}

	// We can also skip the push of the image if it isn't used
	// in any k8s resources! (e.g., it's consumed by another image).

	if cbSkip {
		ps.Printf(ctx, "Skipping push: custom_build() configured to handle push itself")
		return nil
	} else if !IsImageDeployedToK8s(iTarget, kTarget) {
		ps.Printf(ctx, "Skipping push: base image does not need deploy")
		return nil
	} else if ibd.db.WillBuildToKubeContext(ibd.kubeContext) {
		ps.Printf(ctx, "Skipping push: building on cluster's container runtime")
		return nil
	}

	startTime := apis.NowMicro()
	var err error
	if ibd.shouldUseKINDLoad(ctx, iTarget) {
		ps.Printf(ctx, "Loading image to KIND")
		err := ibd.kl.LoadToKIND(ps.AttachLogger(ctx), ref)
		endTime := apis.NowMicro()
		stage := &v1alpha1.DockerImageStageStatus{
			Name:       "kind load",
			StartedAt:  &startTime,
			FinishedAt: &endTime,
		}
		if err != nil {
			stage.Error = fmt.Sprintf("Error loading image to KIND: %v", err)
		}
		return stage
	}

	ps.Printf(ctx, "Pushing with Docker client")
	err = ibd.db.PushImage(ps.AttachLogger(ctx), ref)

	endTime := apis.NowMicro()
	stage := &v1alpha1.DockerImageStageStatus{
		Name:       "docker push",
		StartedAt:  &startTime,
		FinishedAt: &endTime,
	}
	if err != nil {
		stage.Error = fmt.Sprintf("docker push: %v", err)
	}
	return stage
}

func (ibd *ImageBuildAndDeployer) shouldUseKINDLoad(ctx context.Context, iTarg model.ImageTarget) bool {
	isKIND := ibd.env == k8s.EnvKIND5 || ibd.env == k8s.EnvKIND6
	if !isKIND {
		return false
	}

	// if we're using KIND and the image has a separate ref by which it's referred to
	// in the cluster, that implies that we have a local registry in place, and should
	// push to that instead of using KIND load.
	if iTarg.HasDistinctClusterRef() {
		return false
	}

	registry := ibd.k8sClient.LocalRegistry(ctx)
	if !registry.Empty() {
		return false
	}

	return true
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
