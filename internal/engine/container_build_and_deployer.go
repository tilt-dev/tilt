package engine

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

var _ BuildAndDeployer = &ContainerBuildAndDeployer{}

type ContainerBuildAndDeployer struct {
	cu  *build.ContainerUpdater
	env k8s.Env

	// containerBD can't do initial build, so will call out to ibd for that.
	// May also fall back to ibd for certain error cases.
	ibd ImageBuildAndDeployer
}

// TODO(maia): wire this up
func NewContainerBuildAndDeployer(cu *build.ContainerUpdater, env k8s.Env, ibd ImageBuildAndDeployer) *ContainerBuildAndDeployer {
	return &ContainerBuildAndDeployer{
		cu:  cu,
		env: env,
		ibd: ibd,
	}
}

func (cbd *ContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	// TODO(maia): implement containerUpdater-specific stuff!
	return cbd.ibd.BuildAndDeploy(ctx, service, state)
}
