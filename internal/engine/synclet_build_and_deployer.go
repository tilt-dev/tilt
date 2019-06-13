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
	liveUpdateStateSet, err := extractImageTargetsForLiveUpdates(specs, stateSet)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	if len(liveUpdateStateSet) != 1 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("Synclet container builder needs exactly one image target")
	}

	liveUpdateState := liveUpdateStateSet[0]
	iTarget := liveUpdateState.iTarget
	if !isImageDeployedToK8s(iTarget, model.ExtractK8sTargets(specs)) {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("Synclet container builder can only deploy to k8s")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", iTarget.ConfigurationRef.String())
	defer span.Finish()

	return sbd.UpdateInCluster(ctx, liveUpdateState)
}

func (sbd *SyncletBuildAndDeployer) UpdateInCluster(ctx context.Context,
	liveUpdateState liveUpdateStateTree) (store.BuildResultSet, error) {
	var err error
	var changedMappings []build.PathMapping
	var runs []model.Run
	var hotReload bool

	iTarget := liveUpdateState.iTarget
	state := liveUpdateState.iTargetState
	filesChanged := liveUpdateState.filesChanged

	if fbInfo := iTarget.AnyFastBuildInfo(); !fbInfo.Empty() {
		changedMappings, err = build.FilesToPathMappings(filesChanged, fbInfo.Syncs)
		if err != nil {
			return store.BuildResultSet{}, err
		}
		runs = fbInfo.Runs
		hotReload = fbInfo.HotReload
	}
	if luInfo := iTarget.AnyLiveUpdateInfo(); !luInfo.Empty() {
		changedMappings, err = build.FilesToPathMappings(filesChanged, luInfo.SyncSteps())
		if err != nil {
			if pmErr, ok := err.(*build.PathMappingErr); ok {
				// expected error for this builder. One of more files don't match sync's;
				// i.e. they're within the docker context but not within a sync; do a full image build.
				return nil, RedirectToNextBuilderInfof(
					"at least one file (%s) doesn't match a LiveUpdate sync, so performing a full build", pmErr.File)
			}
			return store.BuildResultSet{}, err
		}

		// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
		anyMatch, file, err := luInfo.FallBackOnFiles().AnyMatch(build.PathMappingsToLocalPaths(changedMappings))
		if err != nil {
			return nil, err
		}
		if anyMatch {
			return store.BuildResultSet{}, RedirectToNextBuilderInfof(
				"detected change to fall_back_on file '%s'", file)
		}

		runs = luInfo.RunSteps()
		hotReload = !luInfo.ShouldRestart()
	}

	err = sbd.updateInCluster(ctx, iTarget, state, changedMappings, runs, hotReload)
	if err != nil {
		return store.BuildResultSet{}, err
	}
	return liveUpdateState.createResultSet(), nil
}

func (sbd *SyncletBuildAndDeployer) updateInCluster(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, changedMappings []build.PathMapping, runs []model.Run, hotReload bool) error {
	l := logger.Get(ctx)

	// get files to rm
	toRemove, toArchive, err := build.MissingLocalPaths(ctx, changedMappings)
	if err != nil {
		return errors.Wrap(err, "missingLocalPaths")
	}

	if len(toRemove) > 0 {
		l.Infof("Will delete %d file(s):", len(toRemove))
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	containerPathsToRm := build.PathMappingsToContainerPaths(toRemove)

	// archive files to copy to container
	ab := build.NewArchiveBuilder(ignore.CreateBuildContextFilter(iTarget))
	err = ab.ArchivePathsIfExist(ctx, toArchive)
	if err != nil {
		return errors.Wrap(err, "archivePathsIfExists")
	}
	archive, err := ab.BytesBuffer()
	if err != nil {
		return err
	}
	archivePaths := ab.Paths()

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container:", len(toArchive))
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	deployInfo := state.DeployInfo
	cmds, err := build.BoilRuns(runs, changedMappings)
	if err != nil {
		return err
	}

	// TODO(dbentley): it would be even better to check if the pod has the sidecar
	if sbd.updateMode == UpdateModeKubectlExec || sbd.kCli.ContainerRuntime(ctx) != container.RuntimeDocker {
		if err := sbd.updateViaExec(ctx,
			deployInfo.PodID, deployInfo.Namespace, deployInfo.ContainerName,
			archive, archivePaths, containerPathsToRm, cmds, hotReload); err != nil {
			return err
		}
	} else {
		if err := sbd.updateViaSynclet(ctx,
			deployInfo.PodID, deployInfo.Namespace, deployInfo.ContainerID,
			archive, containerPathsToRm, cmds, hotReload); err != nil {
			return err
		}
	}

	return nil
}

func (sbd *SyncletBuildAndDeployer) updateViaSynclet(ctx context.Context,
	podID k8s.PodID, namespace k8s.Namespace, containerID container.ID,
	archive *bytes.Buffer, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-updateViaSynclet")
	defer span.Finish()
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
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletBuildAndDeployer-updateViaExec")
	defer span.Finish()
	if !hotReload {
		return fmt.Errorf("kubectl exec syncing is only supported on resources that don't use container_restart")
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
			[]string{"tar", "-C", "/", "-x", "-f", "/dev/stdin"}, archive, w, w); err != nil {
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
