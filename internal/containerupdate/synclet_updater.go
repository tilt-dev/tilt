package containerupdate

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/k8s"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type SyncletUpdater struct {
	sm SyncletManager
}

var _ ContainerUpdater = &SyncletUpdater{}

func NewSyncletUpdater(sm SyncletManager) ContainerUpdater {
	return &SyncletUpdater{sm: sm}
}

func (cu *SyncletUpdater) CanUpdateSpecs(specs []model.TargetSpec, env k8s.Env) (canUpd bool, msg string, silent bool) {
	// TODO(maia): implement
	// remember: if you get docker compose specs, error, we can't handle them -- should run with UpdateMode: Container
	return true, "", false
}

func (cu *SyncletUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletUpdater-UpdateContainer")
	defer span.Finish()

	sCli, err := cu.sm.ClientForPod(ctx, deployInfo.PodID, deployInfo.Namespace)
	if err != nil {
		return err
	}

	archiveBytes, err := ioutil.ReadAll(archiveToCopy)
	if err != nil {
		return err
	}

	err = sCli.UpdateContainer(ctx, deployInfo.ContainerID, archiveBytes, filesToDelete, cmds, hotReload)
	if err != nil && build.IsUserBuildFailure(err) {
		return err
	}
	return nil
}
