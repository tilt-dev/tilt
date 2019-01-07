package engine

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerComposeBuildAndDeployer struct {
	dcc dockercompose.DockerComposeClient
}

var _ BuildAndDeployer = &DockerComposeBuildAndDeployer{}

func NewDockerComposeBuildAndDeployer(dcc dockercompose.DockerComposeClient) *DockerComposeBuildAndDeployer {
	return &DockerComposeBuildAndDeployer{
		dcc: dcc,
	}
}

func (bd *DockerComposeBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (br store.BuildResult, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerComposeBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	if !manifest.IsDC() {
		return store.BuildResult{}, RedirectToNextBuilderf("not a docker compose manifest")
	}
	dcInfo := manifest.DCInfo()
	stdout := logger.Get(ctx).Writer(logger.InfoLvl)
	stderr := logger.Get(ctx).Writer(logger.InfoLvl)

	err = bd.dcc.Up(ctx, dcInfo.ConfigPath, manifest.Name.String(), stdout, stderr)
	return store.BuildResult{}, err
}

func (bd *DockerComposeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	return
}
