package engine

import (
	"context"
	"fmt"
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
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-DockerComposeBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	if manifest.DcMeta {
		return store.BuildResult{}, nil
	}

	if manifest.DcServiceName == "" {
		return store.BuildResult{}, CantHandleFailure{fmt.Errorf("no docker compose")}
	}

	cmd := exec.CommandContext(ctx, "docker-compose", "build", manifest.DcServiceName)
	cmd.Stdout = logger.Get(ctx).Writer(logger.InfoLvl)
	cmd.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

	err = cmd.Run()
	err = dockercompose.NiceError(cmd, nil, err)
	return store.BuildResult{}, err
}

func (bd *DockerComposeBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	return
}
