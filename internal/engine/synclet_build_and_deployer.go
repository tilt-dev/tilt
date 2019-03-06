package engine

import (
	"bytes"
	"context"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct {
	sm   SyncletManager
	kCli k8s.Client
}

func NewSyncletBuildAndDeployer(sm SyncletManager, kCli k8s.Client) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		sm:   sm,
		kCli: kCli,
	}
}

func (sbd *SyncletBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	iTargets, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if len(iTargets) != 1 {
		return store.BuildResultSet{}, RedirectToNextBuilderf("Synclet container builder needs exactly one image target")
	}

	iTarget := iTargets[0]
	if !isImageDeployedToK8s(iTarget, extractK8sTargets(specs)) {
		return store.BuildResultSet{}, RedirectToNextBuilderf("Synclet container builder can only deploy to k8s")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", iTarget.Ref.String())
	defer span.Finish()

	state := stateSet[iTarget.ID()]
	return sbd.updateInCluster(ctx, iTarget, state)
}

func (sbd *SyncletBuildAndDeployer) updateInCluster(ctx context.Context,
	image model.ImageTarget, state store.BuildState) (store.BuildResultSet, error) {
	fbInfo := image.MaybeFastBuildInfo()
	paths, err := build.FilesToPathMappings(
		state.FilesChanged(), fbInfo.Mounts)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// archive files to copy to container
	ab := build.NewArchiveBuilder(ignore.CreateBuildContextFilter(image))
	err = ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return store.BuildResultSet{}, errors.Wrap(err, "archivePathsIfExists")
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return store.BuildResultSet{}, err
	}
	archivePaths := ab.Paths()

	// get files to rm
	toRemove, err := build.MissingLocalPaths(ctx, paths)
	if err != nil {
		return store.BuildResultSet{}, errors.Wrap(err, "missingLocalPaths")
	}
	// TODO(maia): can refactor MissingLocalPaths to just return ContainerPaths?
	containerPathsToRm := build.PathMappingsToContainerPaths(toRemove)

	deployInfo := state.DeployInfo
	cmds, err := build.BoilSteps(fbInfo.Steps, paths)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if sbd.kCli.ContainerRuntime(ctx) == container.RuntimeDocker {
		// TODO(dbentley): it would be even better to check if the pod has the sidecar
		if err := sbd.updateViaSynclet(ctx,
			deployInfo.PodID, deployInfo.Namespace, deployInfo.ContainerID,
			archive, containerPathsToRm, cmds, fbInfo.HotReload); err != nil {
			return store.BuildResultSet{}, err
		}
	} else {
		if err := sbd.updateViaExec(ctx,
			deployInfo.PodID, deployInfo.Namespace, deployInfo.ContainerName,
			archive, archivePaths, containerPathsToRm, cmds, fbInfo.HotReload); err != nil {
			return store.BuildResultSet{}, err
		}
	}

	res := state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet)
	res.ContainerID = deployInfo.ContainerID // the container we deployed on top of

	resultSet := store.BuildResultSet{}
	resultSet[image.ID()] = res
	return resultSet, nil
}

func (sbd *SyncletBuildAndDeployer) updateViaSynclet(ctx context.Context,
	podID k8s.PodID, namespace k8s.Namespace, containerID container.ID,
	archive *bytes.Buffer, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	sCli, err := sbd.sm.ClientForPod(ctx, podID, namespace)
	if err != nil {
		return err
	}

	err = sCli.UpdateContainer(ctx, containerID, archive.Bytes(), filesToDelete, cmds, hotReload)
	if err != nil && build.IsUserBuildFailure(err) {
		return WrapDontFallBackError(err)
	}
	return err
}

func (sbd *SyncletBuildAndDeployer) updateViaExec(ctx context.Context,
	podID k8s.PodID, namespace k8s.Namespace, container container.Name,
	archive *bytes.Buffer, archivePaths []string, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	l := logger.Get(ctx)
	w := l.Writer(logger.InfoLvl)

	if len(filesToDelete) > 0 {
		filesToShow := filesToDelete
		if len(filesToShow) > 5 {
			filesToShow = append([]string(nil), filesToDelete[0:5]...)
			filesToShow = append(filesToShow, "...")
		}

		l.Infof("removing %v files %v", len(filesToDelete), filesToShow)
		if err := sbd.kCli.Exec(ctx, podID, container, namespace,
			append([]string{"rm", "-rf"}, filesToDelete...), nil, w, w); err != nil {
			return err
		}
	}

	if len(archivePaths) > 0 {
		filesToShow := archivePaths
		if len(filesToShow) > 5 {
			filesToShow = append([]string(nil), archivePaths[0:5]...)
			filesToShow = append(filesToShow, "...")
		}
		l.Infof("updating %v files %v", len(archivePaths), filesToShow)
		if err := sbd.kCli.Exec(ctx, podID, container, namespace,
			[]string{"tar", "-x", "-f", "/dev/stdin"}, archive, w, w); err != nil {
			return err
		}
	}

	for i, c := range cmds {
		l.Infof("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		if err := sbd.kCli.Exec(ctx, podID, container, namespace,
			c.Argv, nil, w, w); err != nil {
			return WrapDontFallBackError(err)
		}

	}

	return nil
}
