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
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
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
		return store.BuildResultSet{}, RedirectToNextBuilderf(
			"DockerComposeBuildAndDeployer requires exactly one dcTarget (got %d)", len(dcTargets))
	}
	if len(iTargets) > 1 {
		// TODO(nick): Now that we support images depending on other images,
		// we might need to adjust this to support more than one image target.
		return store.BuildResultSet{}, fmt.Errorf(
			"DockerComposeBuildAndDeployer supports at most one ImageTarget (got %d)", len(iTargets))
	}
	dcTarget := dcTargets[0]
	haveImage := len(iTargets) == 1

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerComposeBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", dcTargets[0].Name)
	defer span.Finish()

	results := store.BuildResultSet{}

	if haveImage {
		var err error
		var ref reference.NamedTagged
		iTarget := iTargets[0]
		expectedRef := iTarget.Ref

		ps := build.NewPipelineState(ctx, 1, bd.clock)
		defer func() { ps.End(ctx, err) }()

		// NOTE(maia): we assume that this func takes one DC target and up to one image target
		// corresponding to that service. If this func ever supports specs for more than one
		// service at once, we'll have to match up image build results to DC target by ref.
		ref, err = bd.icb.Build(ctx, iTarget, currentState[iTarget.ID()], ps, true)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		ref, err = bd.tagWithExpected(ctx, ref, expectedRef)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		results[iTarget.ID()] = store.BuildResult{
			Image: ref,
		}
	}

	stdout := logger.Get(ctx).Writer(logger.InfoLvl)
	stderr := logger.Get(ctx).Writer(logger.InfoLvl)
	err := bd.dcc.Up(ctx, dcTarget.ConfigPath, dcTarget.Name, !haveImage, stdout, stderr)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// NOTE(dmiller): right now we only need this the first time. In the future
	// it might be worth it to move this somewhere else
	cid, err := bd.dcc.ContainerID(ctx, dcTarget.ConfigPath, dcTarget.Name)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	results[dcTarget.ID()] = store.BuildResult{
		ContainerID: cid,
	}

	return results, nil
}

func (bd *DockerComposeBuildAndDeployer) tagWithExpected(ctx context.Context, ref reference.NamedTagged,
	expected reference.Named) (reference.NamedTagged, error) {
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
