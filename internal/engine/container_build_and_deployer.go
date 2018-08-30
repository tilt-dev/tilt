package engine

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

var _ BuildAndDeployer = &ContainerBuildAndDeployer{}

type ContainerBuildAndDeployer struct {
	cu        *build.ContainerUpdater
	env       k8s.Env
	k8sClient k8s.Client

	// containerBD can't do initial build, so will call out to ibd for that.
	// May also fall back to ibd for certain error cases.
	ibd ImageBuildAndDeployer
}

func NewContainerBuildAndDeployer(cu *build.ContainerUpdater, env k8s.Env, kCli k8s.Client, ibd ImageBuildAndDeployer) *ContainerBuildAndDeployer {
	return &ContainerBuildAndDeployer{
		cu:        cu,
		env:       env,
		k8sClient: kCli,
		ibd:       ibd,
	}
}

func (cbd *ContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	// TODO(maia): proper output for this stuff

	// TODO(maia): put service.Validate() upstream if we're gonna want to call it regardless
	// of implementation of BuildAndDeploy?
	err := service.Validate()
	if err != nil {
		return BuildResult{}, err
	}

	// ContainerBuildAndDeployer doesn't support initial build; call out to the ImageBuildAndDeployer
	if state.IsEmpty() {
		return cbd.ibd.BuildAndDeploy(ctx, service, state)
	}
	// Service has already been deployed; try to update in the running container

	// If we don't know the pod that we just deployed to/container we just deployed, get it
	if state.LastResult.Container.String() == "" {
		pID, err := cbd.k8sClient.PodWithImage(ctx, state.LastResult.Image)
		if err != nil {
			return BuildResult{}, err
		}
		fmt.Println("pod id:", pID)
		// get containerID from pID (see container_updater.go --> containerIdForPod)
		// attach containerID to build result
	}
	// once have cID -- can call cbd.cu.UpdateContainer(...)

	return BuildResult{}, fmt.Errorf("incremental build via containerBuildAndDeployer")
}
