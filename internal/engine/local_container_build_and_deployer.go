package engine

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/containerupdate"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

var _ BuildAndDeployer = &LocalContainerBuildAndDeployer{}

type LocalContainerBuildAndDeployer struct {
	cu        *containerupdate.DockerContainerUpdater
	analytics *analytics.TiltAnalytics
	env       k8s.Env
}

func NewLocalContainerBuildAndDeployer(cu *containerupdate.DockerContainerUpdater,
	analytics *analytics.TiltAnalytics, env k8s.Env) *LocalContainerBuildAndDeployer {
	return &LocalContainerBuildAndDeployer{
		cu:        cu,
		analytics: analytics,
		env:       env,
	}
}

func (cbd *LocalContainerBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if len(liveUpdateStateSet) != 1 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("Local container builder needs exactly one image target")
	}

	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	isK8s := len(model.ExtractK8sTargets(specs)) > 0
	canLocalUpdate := isDC || (isK8s && cbd.env.IsLocalCluster())
	if !canLocalUpdate {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("Local container builder needs docker-compose or k8s cluster w/ local updates")
	}

	liveUpdateState := liveUpdateStateSet[0]
	iTarget := liveUpdateState.iTarget
	state := liveUpdateState.iTargetState
	filesChanged := liveUpdateState.filesChanged

	span, ctx := opentracing.StartSpanFromContext(ctx, "LocalContainerBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", iTarget.ConfigurationRef.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		cbd.analytics.Timer("build.container", time.Since(startTime), nil)
	}()

	// LocalContainerBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("prev. build state is empty; container build does not support initial deploy")
	}

	var changedFiles []build.PathMapping
	var runs []model.Run
	var hotReload bool

	if fbInfo := iTarget.AnyFastBuildInfo(); !fbInfo.Empty() {
		changedFiles, err = build.FilesToPathMappings(filesChanged, fbInfo.Syncs)
		if err != nil {
			return store.BuildResultSet{}, err
		}
		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	}
	if luInfo := iTarget.AnyLiveUpdateInfo(); !luInfo.Empty() {
		changedFiles, err = build.FilesToPathMappings(filesChanged, luInfo.SyncSteps())
		if err != nil {
			if pmErr, ok := err.(*build.PathMappingErr); ok {
				// expected error for this builder. One of more files don't match sync's;
				// i.e. they're within the docker context but not within a sync; do a full image build.
				return nil, RedirectToNextBuilderInfof(
					"at least one file (%s) doesn't match a LiveUpdate sync, so performing a full build", pmErr.File)
			}
			return store.BuildResultSet{}, err
		}

		// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
		anyMatch, file, err := luInfo.FallBackOnFiles().AnyMatch(build.PathMappingsToLocalPaths(changedFiles))
		if err != nil {
			return nil, err
		}
		if anyMatch {
			return store.BuildResultSet{}, RedirectToNextBuilderInfof(
				"detected change to fall_back_on file '%s'", file)
		}

		runs = luInfo.RunSteps()
		hotReload = !luInfo.ShouldRestart()
	}

	err = cbd.buildAndDeploy(ctx, iTarget, state, changedFiles, runs, hotReload)
	if err != nil {
		return store.BuildResultSet{}, err
	}
	return liveUpdateState.createResultSet(), nil
}

func (cbd *LocalContainerBuildAndDeployer) buildAndDeploy(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, changedFiles []build.PathMapping, runs []model.Run, hotReload bool) error {
	deployInfo := state.DeployInfo
	logger.Get(ctx).Infof("  → Updating container…")
	boiledSteps, err := build.BoilRuns(runs, changedFiles)
	if err != nil {
		return err
	}

	err = cbd.cu.UpdateInContainer(ctx, deployInfo, changedFiles, ignore.CreateBuildContextFilter(iTarget), boiledSteps, hotReload)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return WrapDontFallBackError(err)
		}
		return err
	}
	logger.Get(ctx).Infof("  → Container updated!")
	return nil
}
