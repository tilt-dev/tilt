package engine

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

var _ BuildAndDeployer = &LocalContainerBuildAndDeployer{}

type LocalContainerBuildAndDeployer struct {
	cu        *build.ContainerUpdater
	analytics analytics.Analytics
	env       k8s.Env
}

func NewLocalContainerBuildAndDeployer(cu *build.ContainerUpdater,
	analytics analytics.Analytics, env k8s.Env) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:        cu,
		analytics: analytics,
		env:       env,
	}
}

func (cbd *LocalContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	iTargets, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if len(iTargets) != 1 {
		return store.BuildResultSet{}, RedirectToNextBuilderf("Local container builder needs exactly one image target")
	}

	isDC := len(extractDockerComposeTargets(specs)) > 0
	isK8s := len(extractK8sTargets(specs)) > 0
	canLocalUpdate := isDC || (isK8s && cbd.env.IsLocalCluster())
	if !canLocalUpdate {
		return store.BuildResultSet{}, RedirectToNextBuilderf("Local container builder needs docker-compose or k8s cluster w/ local updates")
	}

	iTarget := iTargets[0]

	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", iTarget.Ref.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		cbd.analytics.Timer("build.container", time.Since(startTime), nil)
	}()

	state := stateSet[iTarget.ID()]

	// LocalContainerBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return store.BuildResultSet{}, RedirectToNextBuilderf("prev. build state is empty; container build does not support initial deploy")
	}

	fbInfo := iTarget.MaybeFastBuildInfo()
	deployInfo := state.DeployInfo
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
