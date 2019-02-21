package engine

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
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
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) != 1 || len(iTargets) != 1 {
		return store.BuildResultSet{}, RedirectToNextBuilderf(
			"SyncletBuildAndDeployer requires exactly one image spec and one k8s deploy spec")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", kTargets[0].Name)
	defer span.Finish()

	iTarget := iTargets[0]
	state := stateSet[iTarget.ID()]
	if err := sbd.canSyncletBuild(ctx, iTarget, state); err != nil {
		return store.BuildResultSet{}, WrapRedirectToNextBuilder(err)
	}

	return sbd.updateInCluster(ctx, iTarget, state)
}

// canSyncletBuild returns an error if we CAN'T build this manifest via the synclet
func (sbd *SyncletBuildAndDeployer) canSyncletBuild(ctx context.Context,
	image model.ImageTarget, state store.BuildState) error {

	// SyncletBuildAndDeployer doesn't support initial build
	if state.IsEmpty() {
		return fmt.Errorf("prev. build state is empty; synclet build does not support initial deploy")
	}

	if !image.IsFastBuild() {
		return fmt.Errorf("container build only supports FastBuilds")
	}

	// Can't do container update if we don't know what container manifest is running in.
	if state.DeployInfo.Empty() {
		return fmt.Errorf("no deploy info")
	}

	return nil
}

func (sbd *SyncletBuildAndDeployer) updateInCluster(ctx context.Context,
	image model.ImageTarget, state store.BuildState) (store.BuildResultSet, error) {
	fbInfo := image.FastBuildInfo()
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

	// get files to rm
	toRemove, err := build.MissingLocalPaths(ctx, paths)
	if err != nil {
		return store.BuildResultSet{}, errors.Wrap(err, "missingLocalPaths")
	}
	// TODO(maia): can refactor MissingLocalPaths to just return ContainerPaths?
	containerPathsToRm := build.PathMappingsToContainerPaths(toRemove)

	deployInfo := state.DeployInfo
	if deployInfo.Empty() {
		return store.BuildResultSet{}, fmt.Errorf("no deploy info")
	}

	cmds, err := build.BoilSteps(fbInfo.Steps, paths)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	res := state.LastResult.ShallowCloneForContainerUpdate(state.FilesChangedSet)
	res.ContainerID = deployInfo.ContainerID // the container we deployed on top of

	resultSet := store.BuildResultSet{}
	resultSet[image.ID()] = res
	return resultSet, nil
}

func (sbd *SyncletBuildAndDeployer) updateViaSynclet(ctx context.Context,
	image model.ImageTarget, state store.BuildState) error {
	sCli, err := sbd.sm.ClientForPod(ctx, deployInfo.PodID, deployInfo.Namespace)
	if err != nil {
		return err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.ContainerID, archive.Bytes(), containerPathsToRm, cmds, fbInfo.HotReload)
	if err != nil {
		if build.IsUserBuildFailure(err) {
			return WrapDontFallBackError(err)
		}
		return err
	}
}

func (sbd *SyncletBuildAndDeployer) updateViaExec(ctx context.Context, podID k8s.PodID, containerID container.ID, tarArchive []byte, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	l := logger.Get(ctx)
	w := l.Writer(logger.InfoLvl)

	if len(filesToDelete) > 0 {
		l.Infof("removing %v files", len(filesToDelete))
		if err := sbc.kCli.Exec(ctx, podID, containerID, namespaceID,
			append([]string{"rm", "-rf"}, filesToDelete...), nil, w, w); err != nil {
			return err
		}
	}

	if len(tarArchive) > 0 {
		l.Infof("updating files")
		if err := sbc.kCli.Exec(ctx, podID, containerID, namespaceID,
			[]string{"tar", "-x", "-f", "/dev/stdin"}, bytes.NewBuffer(tarArchive), w, w); err != nil {
			return err
		}
	}

	for i, c := range cmds {
		log.Printf("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		if err := sbc.kCli.Exec(ctx, podID, containerID, namespaceID,
			c.Argv); err != nil {
			return err
		}

	}
}
