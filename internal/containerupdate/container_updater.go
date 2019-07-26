package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ContainerUpdater interface {
	UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
		archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error
}
