package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

var _ BuildAndDeployer = &LocalContainerBuildAndDeployer{}

const podPollTimeoutLocal = time.Second * 3

type LocalContainerBuildAndDeployer struct {
	cu        *build.ContainerUpdater
	analytics analytics.Analytics
	dd        *DeployDiscovery
}

func NewLocalContainerBuildAndDeployer(cu *build.ContainerUpdater,
	analytics analytics.Analytics, dd *DeployDiscovery) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:        cu,
		analytics: analytics,
		dd:        dd,
	}
}

func (cbd *LocalContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (result store.BuildResult, err error) {
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
		return store.BuildResult{}, err
	}

	// LocalContainerBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return store.BuildResult{}, fmt.Errorf("prev. build state is empty; container build does not support initial deploy")
	}

	if manifest.IsStaticBuild() {
		return store.BuildResult{}, fmt.Errorf("container build does not support static dockerfiles")
	}

	// Otherwise, manifest has already been deployed; try to update in the running container
	deployInfo, err := cbd.dd.DeployInfoForImageBlocking(ctx, state.LastResult.Image)
	if err != nil {
		return store.BuildResult{}, errors.Wrap(err, "deploy info fetch failed")
	} else if deployInfo.Empty() {
		return store.BuildResult{}, fmt.Errorf("no deploy info")
	}

	cf := build.FilesToPathMappings(ctx, state.FilesChanged(), manifest.Mounts)

	logger.Get(ctx).Infof("  → Updating container…")
	boiledSteps, err := build.BoilSteps(manifest.Steps, cf)
	if err != nil {
		return store.BuildResult{}, err
	}

	err = cbd.cu.UpdateInContainer(ctx, deployInfo.containerID, cf, ignore.CreateBuildContextFilter(manifest), boiledSteps)
	if err != nil {
		return store.BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Container updated!")

	return state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet), nil
}

func (cbd *LocalContainerBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-PostProcessBuild")
	span.SetTag("image", result.Image.String())
	defer span.Finish()

	if previousResult.HasImage() && (!result.HasImage() || result.Image != previousResult.Image) {
		_ = cbd.dd.ForgetImage(previousResult.Image)
	}

	if !result.HasImage() {
		// This is normal condition if the previous build failed.
		return
	}

	cbd.dd.EnsureDeployInfoFetchStarted(ctx, result.Image, result.Namespace)
}
