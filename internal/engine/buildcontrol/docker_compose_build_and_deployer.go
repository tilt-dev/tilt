package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmdimage"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockercomposeservice"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockerimage"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type DockerComposeBuildAndDeployer struct {
	dr         *dockerimage.Reconciler
	cr         *cmdimage.Reconciler
	ib         *build.ImageBuilder
	dcsr       *dockercomposeservice.Reconciler
	clock      build.Clock
	ctrlClient ctrlclient.Client
}

var _ BuildAndDeployer = &DockerComposeBuildAndDeployer{}

func NewDockerComposeBuildAndDeployer(
	dr *dockerimage.Reconciler,
	cr *cmdimage.Reconciler,
	ib *build.ImageBuilder,
	dcsr *dockercomposeservice.Reconciler,
	c build.Clock,
	ctrlClient ctrlclient.Client,
) *DockerComposeBuildAndDeployer {
	return &DockerComposeBuildAndDeployer{
		dr:         dr,
		cr:         cr,
		ib:         ib,
		dcsr:       dcsr,
		clock:      c,
		ctrlClient: ctrlClient,
	}
}

// Extract the targets we can apply -- DCBaD supports ImageTargets and DockerComposeTargets.
//
// A given Docker Compose service can be built one of two ways:
//   - Tilt-managed: Tiltfile includes a `docker_build` or `custom_build` directive for the service's image, so Tilt
//     will handle the image lifecycle including building/tagging and Live Update (if configured)
//   - Docker Compose-managed: Building is delegated to Docker Compose via the `--build` flag to the `up` call;
//     Tilt is responsible for watching file changes but does not handle the builds.
//
// It's also possible for a service to reference an image but NOT have any corresponding build (e.g. public/registry
// hosted images are common for infra deps like nginx). These will not have any ImageTarget.
func (bd *DockerComposeBuildAndDeployer) extract(specs []model.TargetSpec) (buildPlan, error) {
	var tiltManagedImageTargets []model.ImageTarget
	var dockerComposeImageTarget *model.ImageTarget
	var dcTargets []model.DockerComposeTarget

	for _, s := range specs {
		switch s := s.(type) {
		case model.ImageTarget:
			if s.IsDockerComposeBuild() {
				if dockerComposeImageTarget != nil {
					return buildPlan{}, DontFallBackErrorf(
						"Target has more than one Docker Compose managed image target")
				}
				dcTarget := s
				dockerComposeImageTarget = &dcTarget
			} else {
				tiltManagedImageTargets = append(tiltManagedImageTargets, s)
			}
		case model.DockerComposeTarget:
			dcTargets = append(dcTargets, s)
		default:
			// unrecognized target
			return buildPlan{}, SilentRedirectToNextBuilderf("DockerComposeBuildAndDeployer does not support target type %T", s)
		}
	}

	if len(dcTargets) != 1 {
		return buildPlan{}, SilentRedirectToNextBuilderf(
			"DockerComposeBuildAndDeployer requires exactly one dcTarget (got %d)", len(dcTargets))
	}

	if len(tiltManagedImageTargets) != 0 && dockerComposeImageTarget != nil {
		return buildPlan{}, DontFallBackErrorf(
			"Docker Compose target cannot have both Tilt-managed and Docker Compose-managed image targets")
	}

	return buildPlan{
		dockerComposeTarget:      dcTargets[0],
		tiltManagedImageTargets:  tiltManagedImageTargets,
		dockerComposeImageTarget: dockerComposeImageTarget,
	}, nil
}

func (bd *DockerComposeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (res store.BuildResultSet, err error) {
	plan, err := bd.extract(specs)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.docker-compose", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	ctx = docker.WithOrchestrator(ctx, model.OrchestratorDC)

	iTargets := plan.tiltManagedImageTargets
	q, err := NewImageTargetQueue(ctx, plan.tiltManagedImageTargets, currentState, bd.ib.CanReuseRef)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// base number of stages is the Tilt-managed image builds + the Docker Compose up step (which might be launching
	// a Tilt-built image OR might build+launch a Docker Compose-managed image)
	numStages := q.CountBuilds() + 1

	reused := q.ReusedResults()
	hasReusedStep := len(reused) > 0
	if hasReusedStep {
		numStages++
	}

	ps := build.NewPipelineState(ctx, numStages, bd.clock)
	defer func() { ps.End(ctx, err) }()

	if hasReusedStep {
		ps.StartPipelineStep(ctx, "Loading cached images")
		for _, result := range reused {
			ps.Printf(ctx, "- %s", store.LocalImageRefFromBuildResult(result))
		}
		ps.EndPipelineStep(ctx)
	}

	imageMapSet := make(map[ktypes.NamespacedName]*v1alpha1.ImageMap, len(plan.dockerComposeTarget.Spec.ImageMaps))
	for _, iTarget := range iTargets {
		if iTarget.IsLiveUpdateOnly {
			continue
		}

		var im v1alpha1.ImageMap
		nn := ktypes.NamespacedName{Name: iTarget.ImageMapName()}
		err := bd.ctrlClient.Get(ctx, nn, &im)
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

		cluster := currentState[target.ID()].ClusterOrEmpty()
		return bd.build(ctx, iTarget, cluster, imageMapSet, ps)
	})

	newResults := q.NewResults().ToBuildResultSet()
	if err != nil {
		return newResults, err
	}

	dcManagedBuild := plan.dockerComposeImageTarget != nil
	var stepName string
	if dcManagedBuild {
		stepName = "Building & deploying"
	} else {
		stepName = "Deploying"
	}
	ps.StartPipelineStep(ctx, stepName)

	dcTarget := plan.dockerComposeTarget
	dcTargetNN := types.NamespacedName{Name: dcTarget.ID().Name.String()}
	status := bd.dcsr.ForceApply(ctx, dcTargetNN, dcTarget.Spec, imageMapSet, dcManagedBuild)
	ps.EndPipelineStep(ctx)
	if status.ApplyError != "" {
		return newResults, fmt.Errorf("%s", status.ApplyError)
	}

	dcTargetID := plan.dockerComposeTarget.ID()
	newResults[dcTargetID] = store.NewDockerComposeDeployResult(dcTargetID, status)
	return newResults, nil
}

func (bd *DockerComposeBuildAndDeployer) build(
	ctx context.Context,
	iTarget model.ImageTarget,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	ps *build.PipelineState) (store.ImageBuildResult, error) {
	switch iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		return bd.dr.ForceApply(ctx, iTarget, cluster, imageMaps, ps)
	case model.CustomBuild:
		return bd.cr.ForceApply(ctx, iTarget, cluster, imageMaps, ps)
	}
	return store.ImageBuildResult{}, fmt.Errorf("invalid image spec")
}

type buildPlan struct {
	dockerComposeTarget model.DockerComposeTarget

	tiltManagedImageTargets []model.ImageTarget

	dockerComposeImageTarget *model.ImageTarget
}
