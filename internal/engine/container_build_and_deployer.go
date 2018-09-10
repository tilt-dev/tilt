package engine

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

var _ BuildAndDeployer = &ContainerBuildAndDeployer{}

type ContainerBuildAndDeployer struct {
	cu        *build.ContainerUpdater
	env       k8s.Env
	k8sClient k8s.Client

	// skipContainer if true, we don't do a container build, and instead do an image build.
	skipContainer bool
}

func DefaultSkipContainer() bool {
	return false
}

func NewContainerBuildAndDeployer(cu *build.ContainerUpdater, env k8s.Env, kCli k8s.Client, skipContainer bool) *ContainerBuildAndDeployer {
	return &ContainerBuildAndDeployer{
		cu:            cu,
		env:           env,
		k8sClient:     kCli,
		skipContainer: skipContainer,
	}
}

func (cbd *ContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerBuildAndDeployer-BuildAndDeploy")
	span.SetTag("service", service.Name.String())
	defer span.Finish()

	if cbd.skipContainer {
		return BuildResult{}, fmt.Errorf("skipContainer == true, won't container build")
	}

	// TODO(maia): proper output for this stuff

	// TODO(maia): put service.Validate() upstream if we're gonna want to call it regardless
	// of implementation of BuildAndDeploy?
	err := service.Validate()
	if err != nil {
		return BuildResult{}, err
	}

	// ContainerBuildAndDeployer doesn't support initial build; call out to the ImageBuildAndDeployer
	if state.IsEmpty() {
		return BuildResult{}, fmt.Errorf("prev. build state is empty; container build does not support initial deploy")
	}

	// Otherwise, service has already been deployed; try to update in the running container

	// (Unless we don't know what container it's running in, in which case we can't.)
	if !state.LastResult.HasContainer() {
		return BuildResult{}, fmt.Errorf("prev. build state has no container")
	}

	cID := state.LastResult.Container
	cf, err := build.FilesToPathMappings(state.FilesChanged(), service.Mounts)
	if err != nil {
		return BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Updating container…")
	err = cbd.cu.UpdateInContainer(ctx, cID, cf, model.BoilStepsTODO(service.Steps))
	if err != nil {
		return BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Container updated!")

	return BuildResult{
		Entities:  state.LastResult.Entities,
		Container: cID,
	}, nil
}

func (cbd *ContainerBuildAndDeployer) GetContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerBuildAndDeployer-GetContainerForBuild")
	defer span.Finish()

	// get pod running the image we just deployed
	pID, err := cbd.k8sClient.PodWithImage(ctx, build.Image)
	if err != nil {
		return "", fmt.Errorf("PodWithImage (img = %s): %v", build.Image, err)
	}

	// get container that's running the app for the pod we found
	cID, err := cbd.cu.ContainerIDForPod(ctx, pID)
	if err != nil {
		return "", fmt.Errorf("ContainerIDForPod (pod = %s): %v", pID, err)
	}

	return cID, nil
}
