package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/ignore"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
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
	deployInfo, ok := sbd.dd.ForgetImage(img)
	if ok {
		return sbd.sm.ForgetPod(ctx, deployInfo.podID)
	}
	return nil
}

func (sbd *SyncletBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-BuildAndDeploy")
	span.SetTag("manifest", manifest.Name.String())
	defer span.Finish()

	// TODO(maia): proper output for this stuff

	if err := sbd.canSyncletBuild(ctx, manifest, state); err != nil {
		return BuildResult{}, err
	}

	return sbd.updateViaSynclet(ctx, manifest, state)
}

// canSyncletBuild returns an error if we CAN'T build this manifest via the synclet
func (sbd *SyncletBuildAndDeployer) canSyncletBuild(ctx context.Context,
	manifest model.Manifest, state BuildState) error {

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
	info, ok := sbd.dd.DeployInfoForImageBlocking(ctx, state.LastResult.Image)
	if !ok {
		return fmt.Errorf("have not yet fetched deploy info for this manifest. " +
			"This should NEVER HAPPEN b/c of the way PostProcessBuild blocks, something is wrong")
	}

	if info.err != nil {
		return fmt.Errorf("no deploy info for this manifest (failed to fetch with error: %v)", info.err)
	}

	return nil
}

func (sbd *SyncletBuildAndDeployer) updateViaSynclet(ctx context.Context,
	manifest model.Manifest, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-updateViaSynclet")
	defer span.Finish()

	paths, err := build.FilesToPathMappings(
		state.FilesChanged(), manifest.Mounts)
	if err != nil {
		return BuildResult{}, err
	}

	// archive files to copy to container
	ab := build.NewArchiveBuilder(ignore.CreateBuildContextFilter(manifest))
	err = ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return BuildResult{}, fmt.Errorf("archivePathsIfExists: %v", err)
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return BuildResult{}, err
	}

	// get files to rm
	toRemove, err := build.MissingLocalPaths(ctx, paths)
	if err != nil {
		return BuildResult{}, fmt.Errorf("missingLocalPaths: %v", err)
	}
	// TODO(maia): can refactor MissingLocalPaths to just return ContainerPaths?
	containerPathsToRm := build.PathMappingsToContainerPaths(toRemove)

	deployInfo, ok := sbd.dd.DeployInfoForImageBlocking(ctx, state.LastResult.Image)
	if !ok || deployInfo == nil {
		// We theoretically already checked this condition :(
		return BuildResult{}, fmt.Errorf("no container ID found for %s (image: %s) "+
			"(should have checked this upstream, something is wrong)",
			manifest.Name, state.LastResult.Image.String())
	}

	cmds, err := build.BoilSteps(manifest.Steps, paths)
	if err != nil {
		return BuildResult{}, err
	}

	sCli, err := sbd.sm.ClientForPod(ctx, deployInfo.podID, state.LastResult.Namespace)
	if err != nil {
		return BuildResult{}, err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.containerID, archive.Bytes(), containerPathsToRm, cmds)
	if err != nil {
		return BuildResult{}, err
	}

	return state.LastResult.ShallowCloneForContainerUpdate(state.filesChangedSet), nil
}

func (sbd *SyncletBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult BuildResult) {
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
