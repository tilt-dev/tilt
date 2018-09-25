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
	ssm SidecarSyncletManager

	kCli k8s.Client

	deployInfo   map[docker.ImgNameAndTag]DeployInfo
	deployInfoMu sync.Mutex
}

type DeployInfo struct {
	podID       k8s.PodID
	containerID k8s.ContainerID
	nodeID      k8s.NodeID
}

func NewSyncletBuildAndDeployer(kCli k8s.Client, ssm SidecarSyncletManager) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		kCli:       kCli,
		deployInfo: make(map[docker.ImgNameAndTag]DeployInfo),
		ssm:        ssm,
	}
}

func (sbd *SyncletBuildAndDeployer) getDeployInfoForImage(img reference.NamedTagged) (DeployInfo, bool) {
	sbd.deployInfoMu.Lock()
	deployInfo, ok := sbd.deployInfo[docker.ToImgNameAndTag(img)]
	sbd.deployInfoMu.Unlock()
	return deployInfo, ok
}

func (sbd *SyncletBuildAndDeployer) setDeployInfoForImage(img reference.NamedTagged, deployInfo DeployInfo) {
	sbd.deployInfoMu.Lock()
	key := docker.ToImgNameAndTag(img)
	sbd.deployInfo[key] = deployInfo
	sbd.deployInfoMu.Unlock()
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

	// Can't do container update if we don't know what container manifest is running in.
	if _, ok := sbd.getDeployInfoForImage(state.LastResult.Image); !ok {
		return fmt.Errorf("no container info for this manifest")
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
	ab := build.NewArchiveBuilder()
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

	deployInfo, ok := sbd.getDeployInfoForImage(state.LastResult.Image)
	if !ok {
		// We theoretically already checked this condition :(
		return BuildResult{}, fmt.Errorf("no container ID found for %s (image: %s)",
			manifest.Name, state.LastResult.Image.String())
	}

	cmds, err := build.BoilSteps(manifest.Steps, paths)
	if err != nil {
		return BuildResult{}, err
	}

	sCli, err := sbd.ssm.ClientForPod(ctx, deployInfo.podID)
	if err != nil {
		return BuildResult{}, err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.containerID, archive.Bytes(), containerPathsToRm, cmds)
	if err != nil {
		return BuildResult{}, err
	}

	return state.LastResult.ShallowCloneForContainerUpdate(state.filesChangedSet), nil
}

func (sbd *SyncletBuildAndDeployer) PostProcessBuild(ctx context.Context, result BuildResult) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-PostProcessBuild")
	span.SetTag("image", result.Image.String())
	defer span.Finish()

	if !result.HasImage() {
		// This is normal if the previous build failed.
		return
	}

	if _, ok := sbd.getDeployInfoForImage(result.Image); !ok {
		deployInfo, err := sbd.getDeployInfo(ctx, result.Image)
		if err != nil {
			// There's a variety of reasons why we might not be able to get the deploy info.
			// The cluster could be in a transient bad state, or the pod
			// could be in a crash loop because the user wrote some code that
			// segfaults. Don't worry too much about it, we'll fall back to an image build.
			logger.Get(ctx).Debugf("failed to get deployInfo: %v", err)
			return
		}
		sbd.setDeployInfoForImage(result.Image, deployInfo)
	}
}

func (sbd *SyncletBuildAndDeployer) getDeployInfo(ctx context.Context, image reference.NamedTagged) (DeployInfo, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-getDeployInfo")
	defer span.Finish()

	// get pod running the image we just deployed
	pod, err := sbd.kCli.PollForPodWithImage(ctx, image, podPollTimeoutSynclet)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "PodWithImage (img = %s)", image)
	}

	pID := k8s.PodIDFromPod(pod)
	nodeID := k8s.NodeIDFromPod(pod)

	// note: this is here both to get sCli for the call to getContainerForBuild below
	// *and* to preemptively set up the tunnel + client
	// (i.e., we'd still want to call this to set up the client even if we were throwing away
	// sCli)
	sCli, err := sbd.ssm.ClientForPod(ctx, pID)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "error getting synclet client for node '%s'", nodeID)
	}

	// get container that's running the app for the pod we found
	cID, err := sCli.ContainerIDForPod(ctx, pID, image)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "syncletClient.GetContainerIdForPod (pod = %s)", pID)
	}

	logger.Get(ctx).Verbosef("talking to synclet client for node %s", nodeID.String())

	return DeployInfo{
		podID:       pID,
		containerID: cID,
		nodeID:      nodeID,
	}, nil
}
