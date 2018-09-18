package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet"
	"google.golang.org/grpc"
)

const podPollTimeoutSynclet = time.Second * 30

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct {
	// NOTE(maia): hacky intermediate SyncletBaD takes a single client,
	// assumes port forwarding a single synclet on <port> -- later, will need
	// a map of NodeID -> syncletClient
	sCli synclet.SyncletClient

	kCli k8s.Client

	deployInfo   map[model.ManifestName]k8s.ContainerID
	deployInfoMu sync.Mutex
}

func DefaultSyncletClient(env k8s.Env) (synclet.SyncletClient, error) {
	if env != k8s.EnvGKE {
		return nil, nil
	}

	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", synclet.Port), grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("connecting to synclet: %v", err)
	}
	cli := synclet.NewGRPCClient(conn)
	return cli, nil
}

func NewSyncletBuildAndDeployer(sCli synclet.SyncletClient, kCli k8s.Client) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		sCli:       sCli,
		kCli:       kCli,
		deployInfo: make(map[model.ManifestName]k8s.ContainerID),
	}
}

func (sbd *SyncletBuildAndDeployer) getContainerIDForManifest(name model.ManifestName) (k8s.ContainerID, bool) {
	sbd.deployInfoMu.Lock()
	cID, ok := sbd.deployInfo[name]
	sbd.deployInfoMu.Unlock()
	return cID, ok
}

func (sbd *SyncletBuildAndDeployer) setContainerIDForManifest(name model.ManifestName, cID k8s.ContainerID) {
	sbd.deployInfoMu.Lock()
	sbd.deployInfo[name] = cID
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
	if _, ok := sbd.getContainerIDForManifest(manifest.Name); !ok {
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

	cID, ok := sbd.getContainerIDForManifest(manifest.Name)
	if !ok {
		// We theoretically already checked this condition :(
		return BuildResult{}, fmt.Errorf("no container ID found for %s", manifest.Name)
	}

	cmds, err := build.BoilSteps(manifest.Steps, paths)
	if err != nil {
		return BuildResult{}, err
	}
	err = sbd.sCli.UpdateContainer(ctx, cID, archive.Bytes(), containerPathsToRm, cmds)
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
	if _, ok := sbd.getContainerIDForManifest(manifest.Name); !ok {
		cID, err := sbd.getContainerForBuild(ctx, result)
		if err != nil {
			logger.Get(ctx).Infof("couldn't get container for %s: %v", manifest.Name, err)
			return
		}
		sbd.setContainerIDForManifest(manifest.Name, cID)
	}
}

func (sbd *SyncletBuildAndDeployer) getContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-getContainerForBuild")
	defer span.Finish()

	// get pod running the image we just deployed
	pID, err := sbd.kCli.PollForPodWithImage(ctx, build.Image, podPollTimeoutSynclet)
	if err != nil {
		return "", fmt.Errorf("PodWithImage (img = %s): %v", build.Image, err)
	}

	// get container that's running the app for the pod we found
	cID, err := sbd.sCli.GetContainerIdForPod(ctx, pID)
	if err != nil {
		return "", fmt.Errorf("syncletClient.GetContainerIdForPod (pod = %s): %v", pID, err)
	}

	return cID, nil
}
