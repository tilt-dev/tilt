package engine

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
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
	span.SetTag("target", iTarget.ConfigurationRef.String())
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

	var changedFiles []build.PathMapping
	var runs []model.Run
	var hotReload bool

	if fbInfo := iTarget.MaybeFastBuildInfo(); fbInfo != nil {
		changedFiles, err = build.FilesToPathMappings(state.FilesChanged(), fbInfo.Syncs)
		if err != nil {
			return store.BuildResultSet{}, err
		}
		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	}
	if luInfo := iTarget.MaybeLiveUpdateInfo(); luInfo != nil {
		tiltRoot := "gettheroot" // PICK UP HERE MONDAY

		changedFiles, err = build.FilesToPathMappings(state.FilesChanged(), luInfo.SyncSteps())
		if err != nil {
			return store.BuildResultSet{}, err
		}

		// If any changed files match a FullRebuildTrigger, fall back to next BuildAndDeployer
		anyMatch, err := ignore.MatchesAnyPaths(luInfo.FullRebuildTriggers, build.PathMappingsToLocalPaths(changedFiles), tiltRoot)
		if err != nil {
			return nil, err
		}
		if anyMatch {
			return store.BuildResultSet{}, RedirectToNextBuilderf(
				"one or more changed files match a FullRebuildTrigger, so will not perform a LiveUpdate")
		}

		runs = luInfo.RunSteps()
		hotReload = !luInfo.ShouldRestart()
	}
	return cbd.buildAndDeploy(ctx, iTarget, state, changedFiles, runs, hotReload)
}

func (cbd *LocalContainerBuildAndDeployer) buildAndDeploy(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, changedFiles []build.PathMapping, runs []model.Run, hotReload bool) (store.BuildResultSet, error) {
	deployInfo := state.DeployInfo
	logger.Get(ctx).Infof("  → Updating container…")
	boiledSteps, err := build.BoilRuns(runs, changedFiles)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// TODO - use PipelineState here when we actually do pipeline output for container builds
	writer := logger.Get(ctx).Writer(logger.InfoLvl)

	err = cbd.cu.UpdateInContainer(ctx, deployInfo.ContainerID, changedFiles, ignore.CreateBuildContextFilter(iTarget), boiledSteps, hotReload, writer)
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
