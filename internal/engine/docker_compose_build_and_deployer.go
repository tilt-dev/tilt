package engine

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerComposeBuildAndDeployer struct {
	dcc dockercompose.DockerComposeClient
	ibd *ImageBuildAndDeployer
}

var _ BuildAndDeployer = &DockerComposeBuildAndDeployer{}

func NewDockerComposeBuildAndDeployer(dcc dockercompose.DockerComposeClient,
	ibd *ImageBuildAndDeployer) *DockerComposeBuildAndDeployer {
	return &DockerComposeBuildAndDeployer{
		dcc: dcc,
		ibd: ibd,
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
		}
	}
	return iTargets, dcTargets
}

func (bd *DockerComposeBuildAndDeployer) BuildAndDeploy(ctx context.Context, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error) {
	iTargets, dcTargets := bd.extract(specs)
	if len(dcTargets) != 1 {
		return store.BuildResultSet{}, RedirectToNextBuilderf(
			"DockerComposeBuildAndDeployer requires exactly one dcTarget (got %d)", len(dcTargets))
	}
	if len(iTargets) > 1 {
		return store.BuildResultSet{}, RedirectToNextBuilderf(
			"DockerComposeBuildAndDeployer supports at most one ImageTarget (got %d)", len(iTargets))
	}
	dcTarget := dcTargets[0]
	haveImage := len(iTargets) == 1

	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerComposeBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", dcTargets[0].Name)
	defer span.Finish()

	if haveImage {
		logger.Get(ctx).Infof("~~ i'm building ur image for: %s", dcTarget.Name)
		var err error
		var ref reference.NamedTagged
		results := store.BuildResultSet{}
		iTarget := iTargets[0]

		ps := build.NewPipelineState(ctx, 1, bd.ibd.clock)
		defer func() { ps.End(ctx, err) }()

		// NOTE(maia): we assume that this func takes one DC target and up to one image target
		// corresponding to that service. If this func ever supports specs for more than one
		// service at once, we'll have to match up image build results to DC target by ref.
		ref, err = bd.ibd.build(ctx, iTarget, currentState[iTarget.ID()], ps, true)
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

	return store.BuildResultSet{}, nil
}
