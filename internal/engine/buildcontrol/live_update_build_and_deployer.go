package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"

	"github.com/tilt-dev/tilt/internal/analytics"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/containerupdate"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ BuildAndDeployer = &LiveUpdateBuildAndDeployer{}

type LiveUpdateBuildAndDeployer struct {
	dcu         *containerupdate.DockerUpdater
	ecu         *containerupdate.ExecUpdater
	updMode     UpdateMode
	kubeContext k8s.KubeContext
	clock       build.Clock
}

func NewLiveUpdateBuildAndDeployer(dcu *containerupdate.DockerUpdater,
	ecu *containerupdate.ExecUpdater,
	updMode UpdateMode,
	kubeContext k8s.KubeContext,
	c build.Clock) *LiveUpdateBuildAndDeployer {
	return &LiveUpdateBuildAndDeployer{
		dcu:         dcu,
		ecu:         ecu,
		updMode:     updMode,
		kubeContext: kubeContext,
		clock:       c,
	}
}

// Info needed to perform a live update
type LiveUpdateInput struct {
	ID model.TargetID

	// Name is human-readable representation of what we're live-updating. The API
	// Server doesn't make any guarantees that this maps to an image name, or a
	// service name, or to anything in particular.
	Name string

	Filter model.PathMatcher
	Spec   v1alpha1.LiveUpdateSpec

	// Derived from KubernetesResource + KubenetesSelector + DockerResource
	Containers []liveupdates.Container

	// Derived from FileWatch + Sync rules
	ChangedFiles []build.PathMapping
}

func (lui LiveUpdateInput) RunSteps() []model.Run {
	return liveupdate.RunSteps(lui.Spec)
}

func (lui LiveUpdateInput) ShouldRestart() bool {
	return liveupdate.ShouldRestart(lui.Spec)
}

func (lui LiveUpdateInput) Empty() bool { return lui.Name == "" }

func (lubad *LiveUpdateBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	containerUpdater := lubad.containerUpdaterForSpecs(specs)
	liveUpdateInputs := make([]LiveUpdateInput, 0, len(liveUpdateStateSet))

	if len(liveUpdateStateSet) == 0 {
		return nil, SilentRedirectToNextBuilderf("no targets for Live Update found")
	}

	for _, luStateTree := range liveUpdateStateSet {
		luInfo, err := liveUpdateInfoForStateTree(luStateTree)
		if err != nil {
			return store.BuildResultSet{}, err
		}

		if !luInfo.Empty() {
			liveUpdateInputs = append(liveUpdateInputs, luInfo)
		}
	}

	ps := build.NewPipelineState(ctx, len(liveUpdateInputs), lubad.clock)
	err = nil
	defer func() {
		ps.End(ctx, err)
	}()

	var dontFallBackErr error
	for _, info := range liveUpdateInputs {
		ps.StartPipelineStep(ctx, "updating image %s", info.Name)
		err = lubad.buildAndDeploy(ctx, ps, containerUpdater, info)
		if err != nil {
			if !IsDontFallBackError(err) {
				// something went wrong, we want to fall back -- bail and
				// let the next builder take care of it
				ps.EndPipelineStep(ctx)
				return store.BuildResultSet{}, err
			}
			// if something went wrong due to USER failure (i.e. run step failed),
			// run the rest of the container updates so all the containers are in
			// a consistent state, then return this error, i.e. don't fall back.
			dontFallBackErr = err
		}
		ps.EndPipelineStep(ctx)
	}

	err = dontFallBackErr
	return createResultSet(liveUpdateStateSet, liveUpdateInputs), err
}

func (lubad *LiveUpdateBuildAndDeployer) buildAndDeploy(ctx context.Context, ps *build.PipelineState, cu containerupdate.ContainerUpdater, info LiveUpdateInput) (err error) {
	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.container", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	containers := info.Containers
	l := logger.Get(ctx)
	cIDStr := container.ShortStrs(liveupdates.IDsForContainers(containers))
	suffix := ""
	if len(containers) != 1 {
		suffix = "(s)"
	}
	ps.StartBuildStep(ctx, "Updating container%s: %s", suffix, cIDStr)

	filter := info.Filter
	runSteps := info.RunSteps()
	changedFiles := info.ChangedFiles
	hotReload := !info.ShouldRestart()
	boiledSteps, err := build.BoilRuns(runSteps, changedFiles)
	if err != nil {
		return err
	}

	// rm files from container
	toRemove, toArchive, err := build.MissingLocalPaths(ctx, changedFiles)
	if err != nil {
		return errors.Wrap(err, "MissingLocalPaths")
	}

	if len(toRemove) > 0 {
		l.Infof("Will delete %d file(s) from container%s: %s", len(toRemove), suffix, cIDStr)
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container%s: %s", len(toArchive), suffix, cIDStr)
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	var lastUserBuildFailure error
	for _, cInfo := range containers {
		archive := build.TarArchiveForPaths(ctx, toArchive, filter)
		err = cu.UpdateContainer(ctx, cInfo, archive,
			build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)
		if err != nil {
			if runFail, ok := build.MaybeRunStepFailure(err); ok {
				// Keep running updates -- we want all containers to have the same files on them
				// even if the Runs don't succeed
				lastUserBuildFailure = err
				logger.Get(ctx).Infof("  → Failed to update container %s: run step %q failed with exit code: %d",
					cInfo.ContainerID.ShortStr(), runFail.Cmd.String(), runFail.ExitCode)
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
				return fmt.Errorf("Failed to update container: container %s successfully updated, "+
					"but last update failed with '%v'", cInfo.ContainerID.ShortStr(), lastUserBuildFailure)
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
func liveUpdateInfoForStateTree(stateTree liveUpdateStateTree) (LiveUpdateInput, error) {
	iTarget := stateTree.iTarget
	filesChanged := stateTree.filesChanged

	var err error
	var fileMappings []build.PathMapping

	luSpec := iTarget.LiveUpdateSpec
	if liveupdate.IsEmptySpec(luSpec) {
		// We should have validated this when generating the LiveUpdateStateTrees, but double check!
		panic(fmt.Sprintf("did not find Live Update info on target %s, "+
			"which should have already been validated for Live Update", iTarget.ID()))
	}

	var pathsMatchingNoSync []string
	fileMappings, pathsMatchingNoSync, err = build.FilesToPathMappings(filesChanged, liveupdate.SyncSteps(luSpec))
	if err != nil {
		return LiveUpdateInput{}, err
	}
	if len(pathsMatchingNoSync) > 0 {
		return LiveUpdateInput{}, RedirectToNextBuilderInfof(
			"Found file(s) not matching any sync for %s (files: %s)", iTarget.ID(),
			ospath.FormatFileChangeList(pathsMatchingNoSync))
	}

	// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
	anyMatch, file, err := liveupdate.FallBackOnFiles(luSpec).AnyMatch(filesChanged)
	if err != nil {
		return LiveUpdateInput{}, err
	}
	if anyMatch {
		prettyFile := ospath.FileDisplayName([]string{luSpec.BasePath}, file)
		return LiveUpdateInput{}, RedirectToNextBuilderInfof(
			"Detected change to fall_back_on file %q", prettyFile)
	}

	if len(fileMappings) == 0 {
		// No files matched a sync for this image, no Live Update to run
		return LiveUpdateInput{}, nil
	}

	return LiveUpdateInput{
		ID:           iTarget.ID(),
		Name:         reference.FamiliarName(iTarget.Refs.ClusterRef()),
		Filter:       ignore.CreateBuildContextFilter(iTarget),
		Spec:         iTarget.LiveUpdateSpec,
		ChangedFiles: fileMappings,
		Containers:   stateTree.containers,
	}, nil
}

func (lubad *LiveUpdateBuildAndDeployer) containerUpdaterForSpecs(specs []model.TargetSpec) containerupdate.ContainerUpdater {
	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	if isDC || lubad.updMode == UpdateModeContainer {
		return lubad.dcu
	}

	if lubad.updMode == UpdateModeKubectlExec {
		return lubad.ecu
	}

	if lubad.dcu.WillBuildToKubeContext(lubad.kubeContext) {
		return lubad.dcu
	}

	return lubad.ecu
}
