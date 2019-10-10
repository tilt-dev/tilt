package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/analytics"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/containerupdate"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
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

func (lui liveUpdInfo) Empty() bool { return lui.iTarget.ID() == model.ImageTarget{}.ID() }

func (lubad *LiveUpdateBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	containerUpdater := lubad.containerUpdaterForSpecs(specs)
	liveUpdInfos := make([]liveUpdInfo, 0, len(liveUpdateStateSet))

	if len(liveUpdateStateSet) == 0 {
		return nil, SilentRedirectToNextBuilderf("no targets for LiveUpdate found")
	}

	for _, luStateTree := range liveUpdateStateSet {
		luInfo, err := liveUpdateInfoForStateTree(luStateTree)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		if !luInfo.Empty() {
			liveUpdInfos = append(liveUpdInfos, luInfo)
		}
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
	return createResultSet(liveUpdateStateSet, liveUpdInfos), dontFallBackErr
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
	l.Infof("  → Updating container(s): %s", cIDStr)

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

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container(s): %s", len(toArchive), cIDStr)
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	var lastUserBuildFailure error
	for _, cInfo := range state.RunningContainers {
		archive := build.TarArchiveForPaths(ctx, toArchive, filter)
		err = cu.UpdateContainer(ctx, cInfo, archive,
			build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)
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
			logger.Get(ctx).Infof("  → Container %s updated!", cInfo.ContainerID.ShortStr())
			if lastUserBuildFailure != nil {
				// This build succeeded, but previously at least one failed due to user error.
				// We may have inconsistent state--bail, and fall back to full build.
				return fmt.Errorf("INCONSISTENT STATE: container %s successfully updated, "+
					"but last update failed with '%v'", cInfo.ContainerID, lastUserBuildFailure)
			}
		}
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
	var fileMappings []build.PathMapping
	var runs []model.Run
	var hotReload bool

	if fbInfo := iTarget.AnyFastBuildInfo(); !fbInfo.Empty() {
		var skipped []string
		fileMappings, skipped, err = build.FilesToPathMappings(filesChanged, fbInfo.Syncs)
		if err != nil {
			return liveUpdInfo{}, err
		}
		if len(skipped) > 0 {
			return liveUpdInfo{}, RedirectToNextBuilderInfof("found file(s) not matching a FastBuild sync, so "+
				"performing a full build. (Files: %s)", strings.Join(skipped, ", "))
		}
		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	} else if luInfo := iTarget.AnyLiveUpdateInfo(); !luInfo.Empty() {
		var skipped []string
		fileMappings, skipped, err = build.FilesToPathMappings(filesChanged, luInfo.SyncSteps())
		if err != nil {
			return liveUpdInfo{}, err
		}
		if len(skipped) > 0 {
			return liveUpdInfo{}, RedirectToNextBuilderInfof("found file(s) not matching a LiveUpdate sync, so "+
				"performing a full build. (Files: %s)", strings.Join(skipped, ", "))
		}

		// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
		anyMatch, file, err := luInfo.FallBackOnFiles().AnyMatch(build.PathMappingsToLocalPaths(fileMappings))
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

	if len(fileMappings) == 0 {
		// No files matched a sync for this image, no LiveUpdate to run
		return liveUpdInfo{}, nil
	}

	return liveUpdInfo{
		iTarget:      iTarget,
		state:        state,
		changedFiles: fileMappings,
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

	if lubad.runtime == container.RuntimeDocker && lubad.env.UsesLocalDockerRegistry() {
		return lubad.dcu
	}

	return lubad.ecu
}
