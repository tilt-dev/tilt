package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/analytics"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/containerupdate"

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

// Info needed to perform a live update
type liveUpdInfo struct {
	iTarget      model.ImageTarget
	state        store.BuildState
	changedFiles []build.PathMapping
	runs         []model.Run
	hotReload    bool
}

func (lubad *LiveUpdateBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	containerUpdater := lubad.containerUpdaterForSpecs(specs)
	liveUpdInfos := make([]liveUpdInfo, len(liveUpdateStateSet))

	if len(liveUpdateStateSet) == 0 {
		return nil, RedirectToNextBuilderInfof("no targets for LiveUpdate found")
	}

	for i, luStateTree := range liveUpdateStateSet {
		luInfo, err := liveUpdateInfoForStateTree(luStateTree)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		liveUpdInfos[i] = luInfo
	}

	var dontFallBackErr error
	for _, info := range liveUpdInfos {
		err = lubad.buildAndDeploy(ctx, containerUpdater, info.iTarget, info.state, info.changedFiles, info.runs, info.hotReload)
		if err != nil {
			if !IsDontFallBackError(err) {
				// something went wrong, we want to fall back -- bail and
				// let the next builder take care of it
				return store.BuildResultSet{}, err
			}
			// if something went wrong due to USER failure (i.e. run step failed),
			// run the rest of the container updates so all the containers are in
			// a consistent state, then return this error, i.e. don't fall back.
			dontFallBackErr = err
		}
	}
	return createResultSet(liveUpdateStateSet), dontFallBackErr
}

func (lubad *LiveUpdateBuildAndDeployer) buildAndDeploy(ctx context.Context, cu containerupdate.ContainerUpdater, iTarget model.ImageTarget, state store.BuildState, changedFiles []build.PathMapping, runs []model.Run, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LiveUpdateBuildAndDeployer-buildAndDeploy")
	span.SetTag("target", iTarget.ConfigurationRef.String())
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.container", time.Since(startTime), nil)
	}()

	l := logger.Get(ctx)
	cIDStr := container.ShortStrs(store.IDsForInfos(state.RunningContainers))
	l.Infof("  → Updating container…")

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
		l.Infof("Will delete %d file(s) from container(s): %s", len(toRemove), cIDStr)
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
	var archiveReader io.Reader = pr

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container(s): %s", len(toArchive), cIDStr)
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	var lastUserBuildFailure error
	for _, cInfo := range state.RunningContainers {
		// always pass a copy of the tar archive reader
		// so multiple updates can access the same data
		var archiveBuf bytes.Buffer
		archiveTee := io.TeeReader(archiveReader, &archiveBuf)

		err = cu.UpdateContainer(ctx, cInfo, archiveTee, build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)
		if err != nil {
			if runFail, ok := build.MaybeRunStepFailure(err); ok {
				// Keep running updates -- we want all containers to have the same files on them
				// even if the Runs don't succeed
				lastUserBuildFailure = err
				logger.Get(ctx).Infof("  → FAILED TO UPDATE CONTAINER %s: run step %q failed with with exit code: %d",
					cInfo.ContainerID, runFail.Cmd.String(), runFail.ExitCode)
				continue
			}

			// Something went wrong with this update and it's NOT the user's fault--
			// likely a infrastructure error. Bail, and fall back to full build.
			return err
		} else {
			logger.Get(ctx).Infof("  → Container %s updated!", cInfo.ContainerID)
			if lastUserBuildFailure != nil {
				// This build succeeded, but previously at least one failed due to user error.
				// We may have inconsistent state--bail, and fall back to full build.
				return fmt.Errorf("INCONSISTENT STATE: container %s successfully updated, "+
					"but last update failed with '%v'", cInfo.ContainerID, lastUserBuildFailure)
			}
		}
		archiveReader = &archiveBuf
	}
	if lastUserBuildFailure != nil {
		return WrapDontFallBackError(lastUserBuildFailure)
	}
	return nil
}

// liveUpdateInfoForStateTree validates the state tree for LiveUpdate and returns
// all the info we need to execute the update.
func liveUpdateInfoForStateTree(stateTree liveUpdateStateTree) (liveUpdInfo, error) {
	iTarget := stateTree.iTarget
	state := stateTree.iTargetState
	filesChanged := stateTree.filesChanged

	var err error
	var changedFiles []build.PathMapping
	var runs []model.Run
	var hotReload bool

	if fbInfo := iTarget.AnyFastBuildInfo(); !fbInfo.Empty() {
		changedFiles, err = build.FilesToPathMappings(filesChanged, fbInfo.Syncs)
		if err != nil {
			return liveUpdInfo{}, err
		}
		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	} else if luInfo := iTarget.AnyLiveUpdateInfo(); !luInfo.Empty() {
		changedFiles, err = build.FilesToPathMappings(filesChanged, luInfo.SyncSteps())
		if err != nil {
			if pmErr, ok := err.(*build.PathMappingErr); ok {
				// expected error for this builder. One of more files don't match sync's;
				// i.e. they're within the docker context but not within a sync; do a full image build.
				return liveUpdInfo{}, RedirectToNextBuilderInfof(
					"at least one file (%s) doesn't match a LiveUpdate sync, so performing a full build", pmErr.File)
			}
			return liveUpdInfo{}, err
		}

		// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
		anyMatch, file, err := luInfo.FallBackOnFiles().AnyMatch(build.PathMappingsToLocalPaths(changedFiles))
		if err != nil {
			return liveUpdInfo{}, err
		}
		if anyMatch {
			return liveUpdInfo{}, RedirectToNextBuilderInfof(
				"detected change to fall_back_on file '%s'", file)
		}

		runs = luInfo.RunSteps()
		hotReload = !luInfo.ShouldRestart()
	} else {
		// We should have validated this when generating the LiveUpdateStateTrees, but double check!
		panic(fmt.Sprintf("found neither FastBuild nor LiveUpdate info on target %s, "+
			"which should have already been validated", iTarget.ID()))
	}

	return liveUpdInfo{
		iTarget:      iTarget,
		state:        state,
		changedFiles: changedFiles,
		runs:         runs,
		hotReload:    hotReload,
	}, nil
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
