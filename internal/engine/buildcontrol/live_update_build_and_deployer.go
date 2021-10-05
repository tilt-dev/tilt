package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	ctrlliveupdate "github.com/tilt-dev/tilt/internal/controllers/core/liveupdate"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ BuildAndDeployer = &LiveUpdateBuildAndDeployer{}

type LiveUpdateBuildAndDeployer struct {
	luReconciler *ctrlliveupdate.Reconciler
	clock        build.Clock
}

func NewLiveUpdateBuildAndDeployer(luReconciler *ctrlliveupdate.Reconciler, c build.Clock) *LiveUpdateBuildAndDeployer {
	return &LiveUpdateBuildAndDeployer{
		luReconciler: luReconciler,
		clock:        c,
	}
}

// Info needed to perform a live update
type LiveUpdateInput struct {
	ID model.TargetID

	// Name is human-readable representation of what we're live-updating. The API
	// Server doesn't make any guarantees that this maps to an image name, or a
	// service name, or to anything in particular.
	Name string

	Spec v1alpha1.LiveUpdateSpec

	ctrlliveupdate.Input
}

func (lui LiveUpdateInput) Empty() bool { return lui.Name == "" }

func (lubad *LiveUpdateBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

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
		err = lubad.buildAndDeploy(ctx, ps, info)
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

func (lubad *LiveUpdateBuildAndDeployer) buildAndDeploy(ctx context.Context, ps *build.PipelineState, info LiveUpdateInput) (err error) {
	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.container", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	containers := info.Containers
	cIDStr := container.ShortStrs(liveupdates.IDsForContainers(containers))
	suffix := ""
	if len(containers) != 1 {
		suffix = "(s)"
	}
	ps.StartBuildStep(ctx, "Updating container%s: %s", suffix, cIDStr)

	status := lubad.luReconciler.ForceApply(
		ctx,
		types.NamespacedName{Name: info.Name},
		info.Spec,
		info.Input)
	if status.UnknownError != nil {
		return status.UnknownError
	}
	if status.ExecError != nil {
		return WrapDontFallBackError(status.ExecError)
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
		ID:   iTarget.ID(),
		Name: reference.FamiliarName(iTarget.Refs.ClusterRef()),
		Spec: iTarget.LiveUpdateSpec,
		Input: ctrlliveupdate.Input{
			Filter:       ignore.CreateBuildContextFilter(iTarget),
			ChangedFiles: fileMappings,
			Containers:   stateTree.containers,
			IsDC:         stateTree.isDC,
		},
	}, nil
}
