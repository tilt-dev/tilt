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

// Extract the targets we can apply into DockerComposeTargets, or nil if we can't apply all targets.
func (bd *DockerComposeBuildAndDeployer) extract(specs []model.TargetSpec) []model.DockerComposeTarget {
	result := []model.DockerComposeTarget{}
	for _, s := range specs {
		dc, isDC := s.(model.DockerComposeTarget)
		if !isDC {
			return nil
		}
		result = append(result, dc)
	}
	return result
}

func (bd *DockerComposeBuildAndDeployer) BuildAndDeploy(ctx context.Context, specs []model.TargetSpec, currentState store.BuildStateSet) (store.BuildResultSet, error) {
	dcs := bd.extract(specs)
	if len(dcs) == 0 {
		return store.BuildResultSet{}, RedirectToNextBuilderf("Specs not supported by DockerComposeBuildAndDeployer")
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "DockerComposeBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", dcs[0].Name)
	defer span.Finish()

	brs := store.BuildResultSet{}
	stdout := logger.Get(ctx).Writer(logger.InfoLvl)
	stderr := logger.Get(ctx).Writer(logger.InfoLvl)
	for _, dc := range dcs {
		err := bd.dcc.Up(ctx, dc.ConfigPath, dc.Name, stdout, stderr)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		cid, err := bd.dcc.ContainerID(ctx, dc.ConfigPath, dc.Name)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		brs[dc.ID()] = store.BuildResult{
			ContainerID: cid,
		}
	}

	return brs, nil
}
