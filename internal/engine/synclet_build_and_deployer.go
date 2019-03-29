package engine

import (
	"bytes"
	"context"
	"fmt"
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
	sm         SyncletManager
	kCli       k8s.Client
	updateMode UpdateMode
}

func NewSyncletBuildAndDeployer(sm SyncletManager, kCli k8s.Client, updateMode UpdateMode) *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{
		sm:         sm,
		kCli:       kCli,
		updateMode: updateMode,
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
	span.SetTag("target", iTarget.ConfigurationRef.String())
	defer span.Finish()

	state := stateSet[iTarget.ID()]
	return sbd.UpdateInCluster(ctx, iTarget, state)
}

func (sbd *SyncletBuildAndDeployer) UpdateInCluster(ctx context.Context,
	iTarget model.ImageTarget, state store.BuildState) (store.BuildResultSet, error) {
	var syncs []model.Mount
	var runs []model.Run
	var hotReload bool

	if fbInfo := iTarget.MaybeFastBuildInfo(); fbInfo != nil {
		syncs = fbInfo.Mounts
		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	}
	if luInfo := iTarget.MaybeLiveUpdateInfo(); luInfo != nil {
		syncs = luInfo.SyncSteps()
		runs = luInfo.RunSteps()
		hotReload = !luInfo.ShouldRestart()
	}
	return sbd.updateInCluster(ctx, iTarget, state, syncs, runs, hotReload)
}

func (sbd *SyncletBuildAndDeployer) updateInCluster(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, syncs []model.Mount, runs []model.Run, hotReload bool) (store.BuildResultSet, error) {
	paths, err := build.FilesToPathMappings(
		state.FilesChanged(), syncs)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// archive files to copy to container
	ab := build.NewArchiveBuilder(ignore.CreateBuildContextFilter(iTarget))
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
	cmds, err := build.BoilRuns(runs, paths)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// TODO(dbentley): it would be even better to check if the pod has the sidecar
	if sbd.updateMode == UpdateModeKubectlExec || sbd.kCli.ContainerRuntime(ctx) != container.RuntimeDocker {
		if err := sbd.updateViaExec(ctx,
			deployInfo.PodID, deployInfo.Namespace, deployInfo.ContainerName,
			archive, archivePaths, containerPathsToRm, cmds, hotReload); err != nil {
			return store.BuildResultSet{}, err
		}
	} else {
		if err := sbd.updateViaSynclet(ctx,
			deployInfo.PodID, deployInfo.Namespace, deployInfo.ContainerID,
			archive, containerPathsToRm, cmds, hotReload); err != nil {
			return store.BuildResultSet{}, err
		}
	}

	res := state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet)
	res.ContainerID = deployInfo.ContainerID // the container we deployed on top of

	resultSet := store.BuildResultSet{}
	resultSet[iTarget.ID()] = res
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
	if !hotReload {
		return fmt.Errorf("kubectl exec syncing is only supported with hotReload set to true")
	}
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
