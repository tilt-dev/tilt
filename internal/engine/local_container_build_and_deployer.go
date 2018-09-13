package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

var _ BuildAndDeployer = &LocalContainerBuildAndDeployer{}

const podPollTimeoutLocal = time.Second * 3

type LocalContainerBuildAndDeployer struct {
	cu        *build.ContainerUpdater
	env       k8s.Env
	k8sClient k8s.Client
	analytics analytics.Analytics
}

func NewLocalContainerBuildAndDeployer(cu *build.ContainerUpdater, env k8s.Env, kCli k8s.Client, analytics analytics.Analytics) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:        cu,
		env:       env,
		k8sClient: kCli,
		analytics: analytics,
	}
}

func (cbd *LocalContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state BuildState) (result BuildResult, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-BuildAndDeploy")
	span.SetTag("manifest", manifest.Name.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		cbd.analytics.Timer("build.container", time.Since(startTime), nil)
	}()

	// TODO(maia): proper output for this stuff

	// TODO(maia): put manifest.Validate() upstream if we're gonna want to call it regardless
	// of implementation of BuildAndDeploy?
	err = manifest.Validate()
	if err != nil {
		return BuildResult{}, err
	}

	// LocalContainerBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return BuildResult{}, fmt.Errorf("prev. build state is empty; container build does not support initial deploy")
	}

	// Otherwise, manifest has already been deployed; try to update in the running container

	// (Unless we don't know what container it's running in, in which case we can't.)
	if !state.LastResult.HasContainer() {
		return BuildResult{}, fmt.Errorf("prev. build state has no container")
	}

	cID := state.LastResult.Container
	cf, err := build.FilesToPathMappings(state.FilesChanged(), manifest.Mounts)
	if err != nil {
		return BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Updating container…")
	boiledSteps, err := build.BoilSteps(manifest.Steps, cf)
	if err != nil {
		return BuildResult{}, err
	}

	err = cbd.cu.UpdateInContainer(ctx, cID, cf, boiledSteps)
	if err != nil {
		return BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Container updated!")

	return BuildResult{
		Entities:  state.LastResult.Entities,
		Container: cID,
	}, nil
}

func (cbd *LocalContainerBuildAndDeployer) PostProcessBuilds(ctx context.Context, states BuildStatesByName) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-PostProcessBuilds")
	defer span.Finish()

	// HACK(maia): give the pod(s) we just deployed a bit to come up.
	// TODO(maia): replace this with polling/smart waiting
	logger.Get(ctx).Infof("Post-processing %d builds...", len(states))

	for serv, state := range states {
		if !state.LastResult.HasImage() {
			logger.Get(ctx).Infof("can't get container for for '%s': BuildResult has no image", serv)
			continue
		}
		if !state.LastResult.HasContainer() {
			cID, err := cbd.getContainerForBuild(ctx, state.LastResult)
			if err != nil {
				logger.Get(ctx).Infof("couldn't get container for %s: %v", serv, err)
				continue
			}
			state.LastResult.Container = cID
			states[serv] = state
		}
	}
}

func (cbd *LocalContainerBuildAndDeployer) getContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-getContainerForBuild")
	defer span.Finish()

	// get pod running the image we just deployed
	pID, err := cbd.k8sClient.PollForPodWithImage(ctx, build.Image, time.Second*3)
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
