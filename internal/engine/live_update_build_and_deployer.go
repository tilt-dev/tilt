package engine

import (
	"context"
	"io"
	"time"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/containerupdate"

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
	dcu     *containerupdate.DockerContainerUpdater
	scu     *containerupdate.SyncletUpdater
	ecu     *containerupdate.ExecUpdater
	updMode UpdateMode
	env     k8s.Env
	runtime container.Runtime
}

func NewLiveUpdateBuildAndDeployer(dcu *containerupdate.DockerContainerUpdater,
	scu *containerupdate.SyncletUpdater, ecu *containerupdate.ExecUpdater,
	updMode UpdateMode, env k8s.Env, runtime container.Runtime) *LiveUpdateBuildAndDeployer {
	return &LiveUpdateBuildAndDeployer{
		dcu:     dcu,
		scu:     scu,
		ecu:     ecu,
		updMode: updMode,
		env:     env,
		runtime: runtime,
	}
}

func (lubad *LiveUpdateBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if len(liveUpdateStateSet) != 1 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("LiveUpdateBuildAndDeployer needs exactly one image target (got %d)", len(liveUpdateStateSet))
	}

	liveUpdateState := liveUpdateStateSet[0]
	iTarget := liveUpdateState.iTarget
	state := liveUpdateState.iTargetState
	filesChanged := liveUpdateState.filesChanged

	span, ctx := opentracing.StartSpanFromContext(ctx, "LiveUpdateBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", iTarget.ConfigurationRef.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.container", time.Since(startTime), nil)
	}()

	// LiveUpdateBuildAndDeployer doesn't support initial build
	// TODO(maia): put this check in extractImageTargetsForLiveUpdates()
	if state.IsEmpty() {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("prev. build state is empty; LiveUpdate does not support initial deploy")
	}

	containerUpdater := lubad.containerUpdaterForSpecs(specs)
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

	err = lubad.buildAndDeploy(ctx, containerUpdater, iTarget, state, changedFiles, runs, hotReload)
	if err != nil {
		return store.BuildResultSet{}, err
	}
	return liveUpdateState.createResultSet(), nil
}

func (lubad *LiveUpdateBuildAndDeployer) buildAndDeploy(ctx context.Context, cu containerupdate.ContainerUpdater, iTarget model.ImageTarget, state store.BuildState, changedFiles []build.PathMapping, runs []model.Run, hotReload bool) error {
	l := logger.Get(ctx)
	l.Infof("  → Updating container…")

	cInfo := state.OneContainerInfo()
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
		l.Infof("Will delete %d file(s) from container: %s", len(toRemove), cInfo.ContainerID.ShortStr())
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
		l.Infof("Will copy %d file(s) to container: %s", len(toArchive), cInfo.ContainerID.ShortStr())
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	err = cu.UpdateContainer(ctx, cInfo, pr, build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return WrapDontFallBackError(err)
		}
		return err
	}
	logger.Get(ctx).Infof("  → Container updated!")
	return nil
}

func (lubad *LiveUpdateBuildAndDeployer) containerUpdaterForSpecs(specs []model.TargetSpec) containerupdate.ContainerUpdater {
	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	if isDC || lubad.updMode == UpdateModeContainer {
		return lubad.dcu
	}

	if lubad.updMode == UpdateModeSynclet {
		return lubad.scu
	}

	if lubad.updMode == UpdateModeKubectlExec {
		return lubad.ecu
	}

	if shouldUseSynclet(lubad.updMode, lubad.env, lubad.runtime) {
		return lubad.scu
	}

	if lubad.runtime == container.RuntimeDocker && lubad.env.IsLocalCluster() {
		return lubad.dcu
	}

	return lubad.ecu
}
