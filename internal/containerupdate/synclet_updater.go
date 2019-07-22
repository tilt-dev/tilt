package containerupdate

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/opentracing/opentracing-go"

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

// SupportsSpecs returns an error (to be surfaced by the BuildAndDeployer) if
// the SyncletUpdater does not support the given specs.
func (cu *SyncletUpdater) SupportsSpecs(specs []model.TargetSpec) (supported bool, msg string) {
	return specsAreOnlyImagesDeployedToK8s(specs, "SyncletUpdater")
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
