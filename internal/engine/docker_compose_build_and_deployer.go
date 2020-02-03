package engine

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type DockerComposeBuildAndDeployer struct {
	dcc   dockercompose.DockerComposeClient
	dc    docker.Client
	icb   *imageAndCacheBuilder
	clock build.Clock
}

var _ BuildAndDeployer = &DockerComposeBuildAndDeployer{}

func NewDockerComposeBuildAndDeployer(dcc dockercompose.DockerComposeClient, dc docker.Client,
	icb *imageAndCacheBuilder, c build.Clock) *DockerComposeBuildAndDeployer {
	return &DockerComposeBuildAndDeployer{
		dcc:   dcc,
		dc:    dc,
		icb:   icb,
		clock: c,
	}
}

// Extract the targets we can apply -- DCBaD supports ImageTargets and DockerComposeTargets.
func (bd *DockerComposeBuildAndDeployer) extract(specs []model.TargetSpec) ([]model.ImageTarget, []model.DockerComposeTarget) {
	var iTargets []model.ImageTarget
	var dcTargets []model.DockerComposeTarget

	for _, s := range specs {
		switch s := s.(type) {
		case model.ImageTarget:
			iTargets = append(iTargets, s)
		case model.DockerComposeTarget:
			dcTargets = append(dcTargets, s)
		default:
			// unrecognized target
			return nil, nil
		}
	}
	return iTargets, dcTargets
}

func (bd *DockerComposeBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error) {
	iTargets, dcTargets := bd.extract(specs)
	if len(dcTargets) != 1 {
		return store.BuildResultSet{}, buildcontrol.SilentRedirectToNextBuilderf(
			"DockerComposeBuildAndDeployer requires exactly one dcTarget (got %d)", len(dcTargets))
	}
	dcTarget := dcTargets[0]

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerComposeBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", dcTargets[0].Name)
	defer span.Finish()

	q, err := buildcontrol.NewImageTargetQueue(ctx, iTargets, currentState, bd.icb.ib.ImageExists)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	numStages := q.CountDirty()
	haveImage := len(iTargets) > 0

	ps := build.NewPipelineState(ctx, numStages, bd.clock)
	defer func() { ps.End(ctx, err) }()

	iTargetMap := model.ImageTargetsByID(iTargets)
	err = q.RunBuilds(func(target model.TargetSpec, state store.BuildState, depResults []store.BuildResult) (store.BuildResult, error) {
		iTarget, ok := target.(model.ImageTarget)
		if !ok {
			return nil, fmt.Errorf("Not an image target: %T", target)
		}

		iTarget, err := injectImageDependencies(iTarget, iTargetMap, depResults)
		if err != nil {
			return nil, err
		}

		expectedRef := iTarget.Refs.ConfigurationRef

		// NOTE(maia): we assume that this func takes one DC target and up to one image target
		// corresponding to that service. If this func ever supports specs for more than one
		// service at once, we'll have to match up image build results to DC target by ref.
		refs, err := bd.icb.Build(ctx, iTarget, currentState[iTarget.ID()], ps)
		if err != nil {
			return nil, err
		}

		ref, err := bd.tagWithExpected(ctx, refs.LocalRef, expectedRef)
		if err != nil {
			return nil, err
		}

		return store.NewImageBuildResultSingleRef(iTarget.ID(), ref), nil
	})

	if err != nil {
		return store.BuildResultSet{}, err
	}

	stdout := logger.Get(ctx).Writer(logger.InfoLvl)
	stderr := logger.Get(ctx).Writer(logger.InfoLvl)
	err = bd.dcc.Up(ctx, dcTarget.ConfigPaths, dcTarget.Name, !haveImage, stdout, stderr)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// NOTE(dmiller): right now we only need this the first time. In the future
	// it might be worth it to move this somewhere else
	cid, err := bd.dcc.ContainerID(ctx, dcTarget.ConfigPaths, dcTarget.Name)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	results := q.Results()
	results[dcTarget.ID()] = store.NewDockerComposeDeployResult(dcTarget.ID(), cid)
	return results, nil
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
