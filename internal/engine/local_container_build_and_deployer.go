package engine

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"

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
}

func NewLocalContainerBuildAndDeployer(cu *build.ContainerUpdater,
	analytics analytics.Analytics) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:        cu,
		analytics: analytics,
	}
}

func (cbd *LocalContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (result store.BuildResult, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-BuildAndDeploy")
	span.SetTag("manifest", manifest.Name.String())
	defer span.Finish()

	if manifest.IsDC() {
		return store.BuildResult{}, RedirectToNextBuilderf("not implemented: DC container builds")
	}

	startTime := time.Now()
	defer func() {
		cbd.analytics.Timer("build.container", time.Since(startTime), nil)
	}()

	// TODO(maia): put manifest.Validate() upstream if we're gonna want to call it regardless
	// of implementation of BuildAndDeploy?
	err = manifest.Validate()
	if err != nil {
		return store.BuildResult{}, WrapDontFallBackError(err)
	}

	// LocalContainerBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return store.BuildResult{}, RedirectToNextBuilderf("prev. build state is empty; container build does not support initial deploy")
	}

	fbInfo := manifest.FastBuildInfo()
	if fbInfo.Empty() {
		return store.BuildResult{}, RedirectToNextBuilderf("container build only supports FastBuilds")
	}

	// Otherwise, manifest has already been deployed; try to update in the running container
	deployInfo := state.DeployInfo
	if deployInfo.Empty() {
		return store.BuildResult{}, RedirectToNextBuilderf("no deploy info")
	}

	cf, err := build.FilesToPathMappings(state.FilesChanged(), fbInfo.Mounts)
	if err != nil {
		return store.BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Updating container…")
	boiledSteps, err := build.BoilSteps(fbInfo.Steps, cf)
	if err != nil {
		return store.BuildResult{}, err
	}

	// TODO - use PipelineState here when we actually do pipeline output for container builds
	writer := logger.Get(ctx).Writer(logger.InfoLvl)

	err = cbd.cu.UpdateInContainer(ctx, deployInfo.ContainerID, cf, ignore.CreateBuildContextFilter(manifest), boiledSteps, writer)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return store.BuildResult{}, WrapDontFallBackError(err)
		}
		return store.BuildResult{}, err
	}
	logger.Get(ctx).Infof("  → Container updated!")

	res := state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet)
	res.ContainerID = deployInfo.ContainerID // the container we deployed on top of
	return res, nil
}

func (cbd *LocalContainerBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
}
