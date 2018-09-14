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

	deployInfo map[model.ManifestName]k8s.ContainerID
}

func NewLocalContainerBuildAndDeployer(cu *build.ContainerUpdater, env k8s.Env, kCli k8s.Client, analytics analytics.Analytics) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:         cu,
		env:        env,
		k8sClient:  kCli,
		analytics:  analytics,
		deployInfo: make(map[model.ManifestName]k8s.ContainerID),
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

	cID, ok := cbd.deployInfo[manifest.Name]
	// (Unless we don't know what container it's running in, in which case we can't.)
	if !ok {
		return BuildResult{}, fmt.Errorf("no container info for this manifest")
	}
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
		Entities: state.LastResult.Entities,
	}, nil
}

func (cbd *LocalContainerBuildAndDeployer) PostProcessBuild(ctx context.Context, manifest model.Manifest, result BuildResult) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-PostProcessBuild")
	span.SetTag("manifest", manifest.Name.String())
	defer span.Finish()

	if !result.HasImage() {
		logger.Get(ctx).Infof("can't get container for for '%s': BuildResult has no image", manifest.Name)
		return
	}
	if _, ok := cbd.deployInfo[manifest.Name]; !ok {
		cID, err := cbd.getContainerForBuild(ctx, result)
		if err != nil {
			logger.Get(ctx).Infof("couldn't get container for %s: %v", manifest.Name, err)
			return
		}
		cbd.deployInfo[manifest.Name] = cID
	}
}

func (cbd *LocalContainerBuildAndDeployer) getContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-getContainerForBuild")
	defer span.Finish()

	// get pod running the image we just deployed
	pID, err := cbd.k8sClient.PollForPodWithImage(ctx, build.Image, podPollTimeoutLocal)
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
