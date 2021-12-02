package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockerimage"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type DockerComposeBuildAndDeployer struct {
	dcc        dockercompose.DockerComposeClient
	dc         docker.Client
	ib         *ImageBuilder
	clock      build.Clock
	ctrlClient ctrlclient.Client
}

var _ BuildAndDeployer = &DockerComposeBuildAndDeployer{}

func NewDockerComposeBuildAndDeployer(dcc dockercompose.DockerComposeClient, dc docker.Client,
	ib *ImageBuilder, c build.Clock,
	ctrlClient ctrlclient.Client) *DockerComposeBuildAndDeployer {
	return &DockerComposeBuildAndDeployer{
		dcc:        dcc,
		dc:         dc,
		ib:         ib,
		clock:      c,
		ctrlClient: ctrlClient,
	}
}

// Extract the targets we can apply -- DCBaD supports ImageTargets and DockerComposeTargets.
//
// A given Docker Compose service can be built one of two ways:
// 	* Tilt-managed: Tiltfile includes a `docker_build` or `custom_build` directive for the service's image, so Tilt
// 		will handle the image lifecycle including building/tagging and Live Update (if configured)
// 	* Docker Compose-managed: Building is delegated to Docker Compose via the `--build` flag to the `up` call;
// 		Tilt is responsible for watching file changes but does not handle the builds.
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
			ref := store.LocalImageRefFromBuildResult(result)
			ps.Printf(ctx, "- %s", container.FamiliarString(ref))
		}
		ps.EndPipelineStep(ctx)
	}

	iTargetMap := model.ImageTargetsByID(plan.tiltManagedImageTargets)
	err = q.RunBuilds(func(target model.TargetSpec, depResults []store.ImageBuildResult) (store.ImageBuildResult, error) {
		iTarget, ok := target.(model.ImageTarget)
		if !ok {
			return store.ImageBuildResult{}, fmt.Errorf("Not an image target: %T", target)
		}

		iTarget, err := InjectImageDependencies(iTarget, iTargetMap, depResults)
		if err != nil {
			return store.ImageBuildResult{}, err
		}

		startTime := apis.NowMicro()
		dockerimage.MaybeUpdateStatus(ctx, bd.ctrlClient, iTarget, dockerimage.ToBuildingStatus(iTarget, startTime))

		expectedRef := iTarget.Refs.ConfigurationRef

		// NOTE(maia): we assume that this func takes one DC target and up to one image target
		// corresponding to that service. If this func ever supports specs for more than one
		// service at once, we'll have to match up image build results to DC target by ref.
		refs, stages, err := bd.ib.Build(ctx, iTarget, ps)
		if err != nil {
			dockerimage.MaybeUpdateStatus(ctx, bd.ctrlClient, iTarget, dockerimage.ToCompletedFailStatus(iTarget, startTime, stages, err))
			return store.ImageBuildResult{}, err
		}
		dockerimage.MaybeUpdateStatus(ctx, bd.ctrlClient, iTarget, dockerimage.ToCompletedSuccessStatus(iTarget, startTime, stages, refs))

		ref, err := bd.tagWithExpected(ctx, refs.LocalRef, expectedRef)
		if err != nil {
			return store.ImageBuildResult{}, err
		}

		return store.NewImageBuildResultSingleRef(iTarget.ID(), ref), nil
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
	stdout := logger.Get(ctx).Writer(logger.InfoLvl)
	stderr := logger.Get(ctx).Writer(logger.InfoLvl)
	err = bd.dcc.Up(ctx, plan.dockerComposeTarget.Spec, dcManagedBuild, stdout, stderr)
	ps.EndPipelineStep(ctx)
	if err != nil {
		return newResults, err
	}

	// NOTE(dmiller): right now we only need this the first time. In the future
	// it might be worth it to move this somewhere else
	cid, err := bd.dcc.ContainerID(ctx, plan.dockerComposeTarget.Spec)
	if err != nil {
		return newResults, err
	}

	// grab the initial container state
	containerJSON, err := bd.dc.ContainerInspect(ctx, string(cid))
	if err != nil {
		logger.Get(ctx).Debugf("Error inspecting container %s: %v", cid, err)
	}

	var containerState *types.ContainerState
	if containerJSON.ContainerJSONBase != nil && containerJSON.ContainerJSONBase.State != nil {
		containerState = containerJSON.ContainerJSONBase.State
	}

	var ports nat.PortMap
	if containerJSON.NetworkSettings != nil {
		ports = containerJSON.NetworkSettings.NetworkSettingsBase.Ports
	}

	dcTargetID := plan.dockerComposeTarget.ID()
	newResults[dcTargetID] = store.NewDockerComposeDeployResult(dcTargetID, cid, containerState, ports)
	return newResults, nil
}

// tagWithExpected tags the given ref as whatever Docker Compose expects, i.e. as
// the `image` value given in docker-compose.yaml. (If DC yaml specifies an image
// with a tag, use that name + tag; otherwise, tag as latest.)
func (bd *DockerComposeBuildAndDeployer) tagWithExpected(ctx context.Context, ref reference.NamedTagged,
	expected container.RefSelector) (reference.NamedTagged, error) {
	var tagAs reference.NamedTagged
	expectedNt, err := container.ParseNamedTagged(expected.String())
	if err == nil {
		// expected ref already includes a tag, so just tag the image as that
		tagAs = expectedNt
	} else {
		// expected ref is just a name, so tag it as `latest` b/c that's what Docker Compose wants
		tagAs, err = reference.WithTag(ref, docker.TagLatest)
		if err != nil {
			return nil, err
		}
	}

	err = bd.dc.ImageTag(ctx, ref.String(), tagAs.String())
	return tagAs, err
}

type buildPlan struct {
	dockerComposeTarget model.DockerComposeTarget

	tiltManagedImageTargets []model.ImageTarget

	dockerComposeImageTarget *model.ImageTarget
}
