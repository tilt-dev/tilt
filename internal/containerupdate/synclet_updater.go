package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type SyncletUpdater struct{}

var _ ContainerUpdater = &SyncletUpdater{}

func NewSyncletUpdater() ContainerUpdater {
	return &SyncletUpdater{}
}

func (cu *SyncletUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	return nil
}
