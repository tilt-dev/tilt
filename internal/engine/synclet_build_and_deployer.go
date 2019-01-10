package engine

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct {
	sm SyncletManager
}

func NewSyncletBuildAndDeployer(sm SyncletManager) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		sm: sm,
	}
}

func (sbd *SyncletBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (store.BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	if manifest.IsDC() {
		return store.BuildResult{}, RedirectToNextBuilderf("not implemented: DC container builds")
	}

	if err := sbd.canSyncletBuild(ctx, manifest, state); err != nil {
		return store.BuildResult{}, WrapRedirectToNextBuilder(err)
	}

	span.SetTag("manifest", manifest.Name.String())
	return sbd.updateViaSynclet(ctx, manifest, state)
}

// canSyncletBuild returns an error if we CAN'T build this manifest via the synclet
func (sbd *SyncletBuildAndDeployer) canSyncletBuild(ctx context.Context,
	manifest model.Manifest, state store.BuildState) error {

	// TODO(maia): put manifest.Validate() upstream if we're gonna want to call it regardless
	// of implementation of BuildAndDeploy?
	err := manifest.Validate()
	if err != nil {
		return err
	}

	// SyncletBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return fmt.Errorf("prev. build state is empty; synclet build does not support initial deploy")
	}

	if !manifest.IsFastBuild() {
		return fmt.Errorf("container build only supports FastBuilds")
	}

	// Can't do container update if we don't know what container manifest is running in.
	if state.DeployInfo.Empty() {
		return fmt.Errorf("no deploy info")
	}

	return nil
}

func (sbd *SyncletBuildAndDeployer) updateViaSynclet(ctx context.Context,
	manifest model.Manifest, state store.BuildState) (store.BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-updateViaSynclet")
	defer span.Finish()

	paths, err := build.FilesToPathMappings(
		state.FilesChanged(), manifest.FastBuildInfo().Mounts)
	if err != nil {
		return store.BuildResult{}, err
	}

	// archive files to copy to container
	ab := build.NewArchiveBuilder(ignore.CreateBuildContextFilter(manifest))
	err = ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return store.BuildResult{}, errors.Wrap(err, "archivePathsIfExists")
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return store.BuildResult{}, err
	}

	// get files to rm
	toRemove, err := build.MissingLocalPaths(ctx, paths)
	if err != nil {
		return store.BuildResult{}, errors.Wrap(err, "missingLocalPaths")
	}
	// TODO(maia): can refactor MissingLocalPaths to just return ContainerPaths?
	containerPathsToRm := build.PathMappingsToContainerPaths(toRemove)

	deployInfo := state.DeployInfo
	if deployInfo.Empty() {
		return store.BuildResult{}, fmt.Errorf("no deploy info")
	}

	cmds, err := build.BoilSteps(manifest.FastBuildInfo().Steps, paths)
	if err != nil {
		return store.BuildResult{}, err
	}

	sCli, err := sbd.sm.ClientForPod(ctx, deployInfo.PodID, deployInfo.Namespace)
	if err != nil {
		return store.BuildResult{}, err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.ContainerID, archive.Bytes(), containerPathsToRm, cmds)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return store.BuildResult{}, WrapDontFallBackError(err)
		}
		return store.BuildResult{}, err
	}

	res := state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet)
	res.ContainerID = deployInfo.ContainerID // the container we deployed on top of
	return res, nil
}
