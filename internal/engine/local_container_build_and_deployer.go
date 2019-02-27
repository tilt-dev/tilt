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

func (cbd *LocalContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) != 1 || len(iTargets) != 1 {
		return store.BuildResultSet{}, RedirectToNextBuilderf(
			"LocalContainerBuildAndDeployer requires exactly one image spec and one k8s deploy spec")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", kTargets[0].Name.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		cbd.analytics.Timer("build.container", time.Since(startTime), nil)
	}()

	iTarget := iTargets[0]
	state := stateSet[iTarget.ID()]

	// LocalContainerBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return store.BuildResultSet{}, RedirectToNextBuilderf("prev. build state is empty; container build does not support initial deploy")
	}

	fbInfo := iTarget.MaybeFastBuildInfo()
	if fbInfo == nil {
		return store.BuildResultSet{}, RedirectToNextBuilderf("container build only supports FastBuilds")
	}

	// Otherwise, manifest has already been deployed; try to update in the running container
	deployInfo := state.DeployInfo
	if deployInfo.Empty() {
		return store.BuildResultSet{}, RedirectToNextBuilderf("no deploy info")
	}

	cf, err := build.FilesToPathMappings(state.FilesChanged(), fbInfo.Mounts)
	if err != nil {
		return store.BuildResultSet{}, err
	}
	logger.Get(ctx).Infof("  → Updating container…")
	boiledSteps, err := build.BoilSteps(fbInfo.Steps, cf)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// TODO - use PipelineState here when we actually do pipeline output for container builds
	writer := logger.Get(ctx).Writer(logger.InfoLvl)

	err = cbd.cu.UpdateInContainer(ctx, deployInfo.ContainerID, cf, ignore.CreateBuildContextFilter(iTarget), boiledSteps, fbInfo.HotReload, writer)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return store.BuildResultSet{}, WrapDontFallBackError(err)
		}
		return store.BuildResultSet{}, err
	}
	logger.Get(ctx).Infof("  → Container updated!")

	res := state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet)
	res.ContainerID = deployInfo.ContainerID // the container we deployed on top of

	resultSet := store.BuildResultSet{}
	resultSet[iTarget.ID()] = res
	return resultSet, nil
}
