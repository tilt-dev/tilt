package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/docker"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

const podPollTimeoutSynclet = time.Second * 30

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct {
	sm SyncletManager

	kCli k8s.Client

	deployInfo   map[docker.ImgNameAndTag]*DeployInfo
	deployInfoMu sync.Mutex
}

type DeployInfo struct {
	podID       k8s.PodID
	containerID k8s.ContainerID
	nodeID      k8s.NodeID

	ready chan struct{} // Close this channel when the DeployInfo is populated
	err   error         // error encountered when populating (if any)
}

func (di *DeployInfo) markReady() { close(di.ready) }
func (di *DeployInfo) waitUntilReady(ctx context.Context) {
	select {
	case <-di.ready:
	case <-ctx.Done():
	}
}

func newEmptyDeployInfo() *DeployInfo {
	return &DeployInfo{ready: make(chan struct{})}
}

func NewSyncletBuildAndDeployer(kCli k8s.Client, sm SyncletManager) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		kCli:       kCli,
		deployInfo: make(map[docker.ImgNameAndTag]*DeployInfo),
		sm:         sm,
	}
}

func (sbd *SyncletBuildAndDeployer) deployInfoForImage(img reference.NamedTagged) (*DeployInfo, bool) {
	sbd.deployInfoMu.Lock()
	defer sbd.deployInfoMu.Unlock()
	deployInfo, ok := sbd.deployInfo[docker.ToImgNameAndTag(img)]
	return deployInfo, ok
}

func (sbd *SyncletBuildAndDeployer) deployInfoForImageOrNew(img reference.NamedTagged) (*DeployInfo, bool) {
	sbd.deployInfoMu.Lock()
	defer sbd.deployInfoMu.Unlock()

	deployInfo, ok := sbd.deployInfo[docker.ToImgNameAndTag(img)]

	if !ok {
		deployInfo = newEmptyDeployInfo()
		sbd.deployInfo[docker.ToImgNameAndTag(img)] = deployInfo
	}
	return deployInfo, ok
}

func (sbd *SyncletBuildAndDeployer) deployInfoForImageBlocking(ctx context.Context, img reference.NamedTagged) (*DeployInfo, bool) {
	deployInfo, ok := sbd.deployInfoForImage(img)
	if deployInfo != nil {
		deployInfo.waitUntilReady(ctx)
	}
	return deployInfo, ok
}

func (sbd *SyncletBuildAndDeployer) forgetImage(ctx context.Context, img reference.NamedTagged) error {
	sbd.deployInfoMu.Lock()
	defer sbd.deployInfoMu.Unlock()

	imgNameAndTag := docker.ToImgNameAndTag(img)

	deployInfo, ok := sbd.deployInfo[imgNameAndTag]
	if !ok {
		return nil
	}

	delete(sbd.deployInfo, imgNameAndTag)

	return sbd.sm.ForgetPod(ctx, deployInfo.podID)
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
	info, ok := sbd.deployInfoForImageBlocking(ctx, state.LastResult.Image)
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
	ab := build.NewArchiveBuilder(manifest.Filter())
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

	deployInfo, ok := sbd.deployInfoForImageBlocking(ctx, state.LastResult.Image)
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

	sCli, err := sbd.sm.ClientForPod(ctx, deployInfo.podID)
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

	info, ok := sbd.deployInfoForImageOrNew(result.Image)
	if ok {
		// This info was already in the map, nothing to do.
		return
	}

	// We just made this info, so populate it. (Can take a while--run async.)
	go func() {
		err := sbd.populateDeployInfo(ctx, result.Image, info)
		if err != nil {
			// There's a variety of reasons why we might not be able to get the deploy info.
			// The cluster could be in a transient bad state, or the pod
			// could be in a crash loop because the user wrote some code that
			// segfaults. Don't worry too much about it, we'll fall back to an image build.
			logger.Get(ctx).Debugf("failed to get deployInfo: %v", err)
			return
		}
	}()
}

func (sbd *SyncletBuildAndDeployer) populateDeployInfo(ctx context.Context, image reference.NamedTagged, info *DeployInfo) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-populateDeployInfo")
	defer span.Finish()

	defer func() {
		info.err = err
		info.markReady()
	}()

	// get pod running the image we just deployed
	pod, err := sbd.kCli.PollForPodWithImage(ctx, image, k8s.DefaultNamespace, podPollTimeoutSynclet)
	if err != nil {
		return errors.Wrapf(err, "PodWithImage (img = %s)", image)
	}

	pID := k8s.PodIDFromPod(pod)
	nodeID := k8s.NodeIDFromPod(pod)

	// Make sure that the deployed image is ready and not crashlooping.
	cID, err := k8s.WaitForContainerReady(ctx, sbd.kCli, pod, image)
	if err != nil {
		return errors.Wrapf(err, "WaitForContainerReady (pod = %s)", pID)
	}

	info.podID = pID
	info.containerID = cID
	info.nodeID = nodeID

	// preemptively set up the tunnel + client
	_, err = sbd.sm.ClientForPod(ctx, pID)
	if err != nil {
		return errors.Wrapf(err, "creating synclet client for pod '%s'", pID)
	}

	return nil
}
