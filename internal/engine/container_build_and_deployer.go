package engine

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

var _ BuildAndDeployer = containerBuildAndDeployer{}

type containerBuildAndDeployer struct {
	cu  build.ContainerUpdater
	env k8s.Env
}

// TODO(maia): wire this up
func NewContainerBuildAndDeployer(cu build.ContainerUpdater, env k8s.Env) BuildAndDeployer {
	return containerBuildAndDeployer{
		cu:  cu,
		env: env,
	}
}

func (cbd containerBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-containerBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	return token, fmt.Errorf("not implemented o_0")
}
