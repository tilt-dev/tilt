package engine

import (
	"context"
	"os/exec"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerComposeBuildAndDeployer struct {
}

var _ BuildAndDeployer = &DockerComposeBuildAndDeployer{}

func NewDockerComposeBuildAndDeployer() *DockerComposeBuildAndDeployer {
	return &DockerComposeBuildAndDeployer{}
}

func (bd *DockerComposeBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (br store.BuildResult, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerComposeBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	if !manifest.IsDockerCompose() {
		return store.BuildResult{}, RedirectToNextBuilderf("not a docker compose manifest")
	}

	cmd := exec.CommandContext(ctx, "docker-compose", "-f", manifest.DcYAMLPath, "up", "--no-deps", "--build", "--force-recreate", "-d", manifest.Name.String())
	cmd.Stdout = logger.Get(ctx).Writer(logger.InfoLvl)
	cmd.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

	err = cmd.Run()
	err = dockercompose.FormatError(cmd, nil, err)
	return store.BuildResult{}, err
}

func (bd *DockerComposeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	return
}
