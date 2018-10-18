package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

const podPollTimeoutSynclet = time.Second * 30

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct {
	sm SyncletManager
	dd *DeployDiscovery
}

func NewSyncletBuildAndDeployer(dd *DeployDiscovery, sm SyncletManager) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		dd: dd,
		sm: sm,
	}
}

func (sbd *SyncletBuildAndDeployer) forgetImage(ctx context.Context, img reference.NamedTagged) error {
	deployInfo := sbd.dd.ForgetImage(img)
	if deployInfo.podID != "" {
		return sbd.sm.ForgetPod(ctx, deployInfo.podID)
	}
	return nil
}

func (sbd *SyncletBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (store.BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-BuildAndDeploy")
	span.SetTag("manifest", manifest.Name.String())
	defer span.Finish()

	// TODO(maia): proper output for this stuff

	if err := sbd.canSyncletBuild(ctx, manifest, state); err != nil {
		return store.BuildResult{}, err
	}

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

	if manifest.IsStaticBuild() {
		return fmt.Errorf("container build does not support static dockerfiles")
	}

	// Can't do container update if we don't know what container manifest is running in.
	info, err := sbd.dd.DeployInfoForImageBlocking(ctx, state.LastResult.Image)
	if err != nil {
		return errors.Wrap(err, "deploy info fetch failed")
	} else if info.Empty() {
		return fmt.Errorf("no deploy info")
	}

	return nil
}

func (sbd *SyncletBuildAndDeployer) updateViaSynclet(ctx context.Context,
	manifest model.Manifest, state store.BuildState) (store.BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-updateViaSynclet")
	defer span.Finish()

	paths := build.FilesToPathMappings(ctx, state.FilesChanged(), manifest.Mounts)

	// archive files to copy to container
	ab := build.NewArchiveBuilder(ignore.CreateBuildContextFilter(manifest))
	err := ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return store.BuildResult{}, fmt.Errorf("archivePathsIfExists: %v", err)
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return store.BuildResult{}, err
	}

	// get files to rm
	toRemove, err := build.MissingLocalPaths(ctx, paths)
	if err != nil {
		return store.BuildResult{}, fmt.Errorf("missingLocalPaths: %v", err)
	}
	// TODO(maia): can refactor MissingLocalPaths to just return ContainerPaths?
	containerPathsToRm := build.PathMappingsToContainerPaths(toRemove)

	deployInfo, err := sbd.dd.DeployInfoForImageBlocking(ctx, state.LastResult.Image)

	// We theoretically already checked this condition :(
	if err != nil {
		return store.BuildResult{}, errors.Wrap(err, "deploy info fetch failed")
	} else if deployInfo.Empty() {
		return store.BuildResult{}, fmt.Errorf("no deploy info")
	}

	cmds, err := build.BoilSteps(manifest.Steps, paths)
	if err != nil {
		return store.BuildResult{}, err
	}

	sCli, err := sbd.sm.ClientForPod(ctx, deployInfo.podID, state.LastResult.Namespace)
	if err != nil {
		return store.BuildResult{}, err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.containerID, archive.Bytes(), containerPathsToRm, cmds)
	if err != nil {
		return store.BuildResult{}, err
	}

	return state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet), nil
}

func (sbd *SyncletBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-PostProcessBuild")
	span.SetTag("image", result.Image.String())
	defer span.Finish()

	if previousResult.HasImage() && (!result.HasImage() || result.Image != previousResult.Image) {
		err := sbd.forgetImage(ctx, previousResult.Image)
		if err != nil {
			logger.Get(ctx).Debugf("failed to get clean up image-related state: %v", err)
		}
	}

	if !result.HasImage() {
		// This is normal if the previous build failed.
		return
	}

	sbd.dd.EnsureDeployInfoFetchStarted(ctx, result.Image, result.Namespace)
}
