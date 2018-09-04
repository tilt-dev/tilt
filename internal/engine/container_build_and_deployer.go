package engine

import (
	"context"

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

	// containerBD can't do initial build, so will call out to ibd for that.
	// May also fall back to ibd for certain error cases.
	ibd ImageBuildAndDeployer

	// skipContainer if true, we don't do a container build, and instead do an image build.
	skipContainer bool
}

func DefaultSkipContainer() bool {
	return true
}

func NewContainerBuildAndDeployer(cu *build.ContainerUpdater, env k8s.Env, kCli k8s.Client, ibd ImageBuildAndDeployer, skipContainer bool) *ContainerBuildAndDeployer {
	return &ContainerBuildAndDeployer{
		cu:            cu,
		env:           env,
		k8sClient:     kCli,
		ibd:           ibd,
		skipContainer: skipContainer,
	}
}

func (cbd *ContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()
	if cbd.skipContainer {
		return cbd.ibd.BuildAndDeploy(ctx, service, state)
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
		return cbd.ibd.BuildAndDeploy(ctx, service, state)
	}
	// Service has already been deployed; try to update in the running container

	// If we don't know the pod that we just deployed to/container we just deployed, get it
	var cID k8s.ContainerID
	if state.LastResult.Container.String() == "" {
		pID, err := cbd.k8sClient.PodWithImage(ctx, state.LastResult.Image)
		if err != nil {
			logger.Get(ctx).Infof("Unable to find pod, falling back to image deploy: %s", err)
			return cbd.ibd.BuildAndDeploy(ctx, service, state)
		}
		logger.Get(ctx).Infof("Deploying to pod: %s", pID)
		// get containerID from pID (see container_updater.go --> containerIdForPod)
		cID, err = cbd.cu.ContainerIDForPod(ctx, pID)
		if err != nil {
			logger.Get(ctx).Infof("Unable to find container, falling back to image deploy: %s", err)
			return cbd.ibd.BuildAndDeploy(ctx, service, state)
		}
		logger.Get(ctx).Infof("Got container ID for pod: %s", cID.ShortStr())
	} else {
		cID = state.LastResult.Container
	}
	// once have cID -- can call cbd.cu.UpdateContainer(...)
	cf, err := build.FilesToPathMappings(state.FilesChanged(), service.Mounts)
	if err != nil {
		return BuildResult{}, err
	}
	logger.Get(ctx).Infof("Updating container...")
	err = cbd.cu.UpdateInContainer(ctx, cID, cf, service.Steps)
	if err != nil {
		logger.Get(ctx).Infof("Unable to update container, falling back to image deploy: %s", err)
		return cbd.ibd.BuildAndDeploy(ctx, service, state)
	}
	logger.Get(ctx).Infof("Container updated")

	return BuildResult{
		Entities:  state.LastResult.Entities,
		Container: cID,
	}, nil
}
