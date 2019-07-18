package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ExecUpdater struct{}

var _ ContainerUpdater = &ExecUpdater{}

func NewExecUpdater() ContainerUpdater {
	return &ExecUpdater{}
}

func (cu *ExecUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	return nil
}
