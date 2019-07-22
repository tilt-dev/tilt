package engine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"

	errors2 "github.com/windmilleng/tilt/internal/engine/errors"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/target"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

var _ BuildAndDeployer = &LiveUpdateBuildAndDeployer{}

type LiveUpdateBuildAndDeployer struct {
	cu  containerupdate.ContainerUpdater
	env k8s.Env
}

// Differentiate between the LiveUpdateBaD we'll use for k8s updates (may have
// ContainerUpdater = docker || synclet || exec) and the one we'll use for
// DC updates (will always have DockerContainerUpdater)
type k8sLiveUpdBAD *LiveUpdateBuildAndDeployer
type dcLiveUpdBAD *LiveUpdateBuildAndDeployer

func NewLiveUpdateBuildAndDeployer(cu containerupdate.ContainerUpdater, env k8s.Env) *LiveUpdateBuildAndDeployer {
	return &LiveUpdateBuildAndDeployer{
		cu:  cu,
		env: env,
	}
}

func ProvideLiveUpdateBuildAndDeployerForK8s(k8scu containerupdate.K8sContainerUpdater, env k8s.Env) k8sLiveUpdBAD {
	return NewLiveUpdateBuildAndDeployer(k8scu, env)
}

func ProvideLiveUpdateBuildAndDeployerForDC(dccu containerupdate.DCContainerUpdater, env k8s.Env) dcLiveUpdBAD {
	return NewLiveUpdateBuildAndDeployer(dccu, env)
}

func (lubad LiveUpdateBuildAndDeployer) IsSyncletUpdater() bool {
	_, ok := lubad.cu.(*containerupdate.SyncletUpdater)
	return ok
}

func (lubad *LiveUpdateBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := target.ExtractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if len(liveUpdateStateSet) != 1 {
		return store.BuildResultSet{}, errors2.SilentRedirectToNextBuilderf("LiveUpdateBuildAndDeployer needs exactly one image target (got %d)", len(liveUpdateStateSet))
	}

	supported, msg := lubad.cu.SupportsSpecs(specs)
	if !supported {
		return store.BuildResultSet{}, fmt.Errorf(msg)
	}

	liveUpdateState := liveUpdateStateSet[0]
	iTarget := liveUpdateState.ITarget
	state := liveUpdateState.ITargetState
	filesChanged := liveUpdateState.FilesChanged

	span, ctx := opentracing.StartSpanFromContext(ctx, "LiveUpdateBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", iTarget.ConfigurationRef.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.container", time.Since(startTime), nil)
	}()

	// LiveUpdateBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return store.BuildResultSet{}, errors2.SilentRedirectToNextBuilderf("prev. build state is empty; LiveUpdate does not support initial deploy")
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
				return nil, errors2.RedirectToNextBuilderInfof(
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
			return store.BuildResultSet{}, errors2.RedirectToNextBuilderInfof(
				"detected change to fall_back_on file '%s'", file)
		}

		runs = luInfo.RunSteps()
		hotReload = !luInfo.ShouldRestart()
	}

	err = lubad.buildAndDeploy(ctx, iTarget, state, changedFiles, runs, hotReload)
	if err != nil {
		return store.BuildResultSet{}, err
	}
	return liveUpdateState.CreateResultSet(), nil
}

func (lubad *LiveUpdateBuildAndDeployer) buildAndDeploy(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, changedFiles []build.PathMapping, runs []model.Run, hotReload bool) error {
	l := logger.Get(ctx)
	l.Infof("  → Updating container…")

	deployInfo := state.DeployInfo
	filter := ignore.CreateBuildContextFilter(iTarget)
	boiledSteps, err := build.BoilRuns(runs, changedFiles)
	if err != nil {
		return err
	}

	// rm files from container
	toRemove, toArchive, err := build.MissingLocalPaths(ctx, changedFiles)
	if err != nil {
		return errors.Wrap(err, "MissingLocalPaths")
	}

	if len(toRemove) > 0 {
		l.Infof("Will delete %d file(s) from container: %s", len(toRemove), deployInfo.ContainerID.ShortStr())
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	// copy files to container
	pr, pw := io.Pipe()
	go func() {
		ab := build.NewArchiveBuilder(pw, filter)
		err = ab.ArchivePathsIfExist(ctx, toArchive)
		if err != nil {
			_ = pw.CloseWithError(errors.Wrap(err, "archivePathsIfExists"))
		} else {
			_ = ab.Close()
			_ = pw.Close()
		}
	}()

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container: %s", len(toArchive), deployInfo.ContainerID.ShortStr())
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	err = lubad.cu.UpdateContainer(ctx, deployInfo, pr, build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return errors2.WrapDontFallBackError(err)
		}
		return err
	}
	logger.Get(ctx).Infof("  → Container updated!")
	return nil
}
