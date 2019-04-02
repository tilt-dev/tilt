package engine

import (
	"context"
	"fmt"
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
	cf := state.FilesChanged()

	var cfMappings []build.PathMapping
	var runs []model.Run
	var hotReload bool

	if fbInfo := iTarget.MaybeFastBuildInfo(); fbInfo != nil {
		cfMappings, err = build.FilesToPathMappings(cf, fbInfo.Syncs)
		if err != nil {
			return store.BuildResultSet{}, err
		}
		if len(cfMappings) != len(cf) {
			return nil, fmt.Errorf("failed to match one of more of changed files %v with a sync: %+v", cf, fbInfo.Syncs)
		}

		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	}
	if luInfo := iTarget.MaybeLiveUpdateInfo(); luInfo != nil {
		cfMappings, err = build.FilesToPathMappings(cf, luInfo.SyncSteps())
		if err != nil {
			return store.BuildResultSet{}, err
		}
		if len(cfMappings) != len(cf) {
			return nil, RedirectToNextBuilderf("one or more changed files do NOT match a LiveUpdate sync, " +
				"so should perform a base build")
		}

		// If any changed files match a FullRebuildTrigger, fall back to next BuildAndDeployer
		anyMatch, err := luInfo.FullRebuildTriggers.AnyMatch(build.PathMappingsToLocalPaths(cfMappings))
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
	return cbd.buildAndDeploy(ctx, iTarget, state, cfMappings, runs, hotReload)
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
