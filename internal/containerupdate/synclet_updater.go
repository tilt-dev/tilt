package containerupdate

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type SyncletUpdater struct {
	sm SyncletManager
}

var _ ContainerUpdater = &SyncletUpdater{}

func NewSyncletUpdater(sm SyncletManager) *SyncletUpdater {
	return &SyncletUpdater{sm: sm}
}

func (cu *SyncletUpdater) UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SyncletUpdater-UpdateContainer")
	defer span.Finish()

	sCli, err := cu.sm.ClientForPod(ctx, cInfo.PodID, cInfo.Namespace)
	if err != nil {
		return err
	}

	archiveBytes, err := ioutil.ReadAll(archiveToCopy)
	if err != nil {
		return err
	}

	return sCli.UpdateContainer(ctx, cInfo.ContainerID, archiveBytes, filesToDelete, cmds, hotReload)
}
