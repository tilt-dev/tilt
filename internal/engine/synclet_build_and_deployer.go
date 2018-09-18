package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

const podPollTimeoutSynclet = time.Second * 30

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct {
	syncletClientManager SyncletClientManager

	kCli k8s.Client

	deployInfo   map[model.ManifestName]DeployInfo
	deployInfoMu sync.Mutex
}

type DeployInfo struct {
	containerID k8s.ContainerID
	nodeID      k8s.NodeID
}

func NewSyncletBuildAndDeployer(kCli k8s.Client, scm SyncletClientManager) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		kCli:                 kCli,
		deployInfo:           make(map[model.ManifestName]DeployInfo),
		syncletClientManager: scm,
	}
}

func (sbd *SyncletBuildAndDeployer) getDeployInfoForManifest(name model.ManifestName) (DeployInfo, bool) {
	sbd.deployInfoMu.Lock()
	deployInfo, ok := sbd.deployInfo[name]
	sbd.deployInfoMu.Unlock()
	return deployInfo, ok
}

func (sbd *SyncletBuildAndDeployer) setDeployInfoForManifest(name model.ManifestName, deployInfo DeployInfo) {
	sbd.deployInfoMu.Lock()
	sbd.deployInfo[name] = deployInfo
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
	if _, ok := sbd.getDeployInfoForManifest(manifest.Name); !ok {
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

	deployInfo, ok := sbd.getDeployInfoForManifest(manifest.Name)
	if !ok {
		// We theoretically already checked this condition :(
		return BuildResult{}, fmt.Errorf("no container ID found for %s", manifest.Name)
	}

	cmds, err := build.BoilSteps(manifest.Steps, paths)
	if err != nil {
		return BuildResult{}, err
	}

	sCli, err := sbd.syncletClientManager.ClientForNode(ctx, deployInfo.nodeID)
	if err != nil {
		return BuildResult{}, err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.containerID, archive.Bytes(), containerPathsToRm, cmds)
	if err != nil {
		return BuildResult{}, err
	}

	return state.LastResult.ShallowCloneForContainerUpdate(state.filesChangedSet), nil
}

func (sbd *SyncletBuildAndDeployer) PostProcessBuild(ctx context.Context, manifest model.Manifest, result BuildResult) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-PostProcessBuild")
	span.SetTag("manifest", manifest.Name.String())
	defer span.Finish()

	if !result.HasImage() {
		logger.Get(ctx).Infof("can't get container for for '%s': BuildResult has no image", manifest.Name)
		return
	}
	if _, ok := sbd.getDeployInfoForManifest(manifest.Name); !ok {
		deployInfo, err := sbd.getDeployInfo(ctx, result.Image)
		if err != nil {
			logger.Get(ctx).Infof("failed to get deployInfo: %v", err)
			return
		}
		sbd.setDeployInfoForManifest(manifest.Name, deployInfo)
	}
}

func (sbd *SyncletBuildAndDeployer) getDeployInfo(ctx context.Context, image reference.NamedTagged) (DeployInfo, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-getDeployInfo")
	defer span.Finish()

	// get pod running the image we just deployed
	pID, err := sbd.kCli.PollForPodWithImage(ctx, image, podPollTimeoutSynclet)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "PodWithImage (img = %s)", image)
	}

	nodeID, err := sbd.kCli.GetNodeForPod(ctx, pID)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "couldn't get node for pod '%s'", pID.String())
	}

	// note: this is here both to get sCli for the call to getContainerForBuild below
	// *and* to preemptively set up the tunnel + client
	// (i.e., we'd still want to call this to set up the client even if we were throwing away
	// sCli)
	sCli, err := sbd.syncletClientManager.ClientForNode(ctx, nodeID)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "error getting synclet client for node '%s'", nodeID)
	}

	// get container that's running the app for the pod we found
	cID, err := sCli.GetContainerIdForPod(ctx, pID)
	if err != nil {
		return DeployInfo{}, errors.Wrapf(err, "syncletClient.GetContainerIdForPod (pod = %s)", pID)
	}

	logger.Get(ctx).Verbosef("talking to synclet client for node %s", nodeID.String())

	return DeployInfo{cID, nodeID}, nil
}
