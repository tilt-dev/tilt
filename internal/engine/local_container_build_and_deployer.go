package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

var _ BuildAndDeployer = &LocalContainerBuildAndDeployer{}

const podPollTimeoutLocal = time.Second * 3

type LocalContainerBuildAndDeployer struct {
	cu        *build.ContainerUpdater
	cr        *build.ContainerResolver
	env       k8s.Env
	k8sClient k8s.Client
	analytics analytics.Analytics

	deployInfo   map[docker.ImgNameAndTag]k8s.ContainerID
	deployInfoMu sync.Mutex
}

func NewLocalContainerBuildAndDeployer(cu *build.ContainerUpdater, cr *build.ContainerResolver,
	env k8s.Env, kCli k8s.Client, analytics analytics.Analytics) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:         cu,
		cr:         cr,
		env:        env,
		k8sClient:  kCli,
		analytics:  analytics,
		deployInfo: make(map[docker.ImgNameAndTag]k8s.ContainerID),
	}
}

func (cbd *LocalContainerBuildAndDeployer) getContainerIDForImage(img reference.NamedTagged) (k8s.ContainerID, bool) {
	cbd.deployInfoMu.Lock()
	cID, ok := cbd.deployInfo[docker.ToImgNameAndTag(img)]
	cbd.deployInfoMu.Unlock()
	return cID, ok
}

func (cbd *LocalContainerBuildAndDeployer) setContainerIDForImage(img reference.NamedTagged, cID k8s.ContainerID) {
	cbd.deployInfoMu.Lock()
	key := docker.ToImgNameAndTag(img)
	cbd.deployInfo[key] = cID
	cbd.deployInfoMu.Unlock()
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
	cID, ok := cbd.getContainerIDForImage(state.LastResult.Image)

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

	return state.LastResult.ShallowCloneForContainerUpdate(state.filesChangedSet), nil
}

func (cbd *LocalContainerBuildAndDeployer) PostProcessBuild(ctx context.Context, result BuildResult) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-PostProcessBuild")
	span.SetTag("image", result.Image.String())
	defer span.Finish()

	if !result.HasImage() {
		// This is normal condition if the previous build failed.
		return
	}

	if _, ok := cbd.getContainerIDForImage(result.Image); !ok {
		cID, err := cbd.getContainerForBuild(ctx, result)
		if err != nil {
			// There's a variety of reasons why we might not be able to get a
			// container.  The cluster could be in a transient bad state, or the pod
			// could be in a crash loop because the user wrote some code that
			// segfaults. Don't worry too much about it, we'll fall back to an image build.
			logger.Get(ctx).Debugf("couldn't get container for img %s: %v", result.Image.String(), err)
			return
		}
		cbd.setContainerIDForImage(result.Image, cID)
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
	cID, err := cbd.cr.ContainerIDForPod(ctx, pID, build.Image)
	if err != nil {
		return "", fmt.Errorf("ContainerIDForPod (pod = %s): %v", pID, err)
	}

	return cID, nil
}
