package containerupdate

import (
	"context"
	"io"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ContainerUpdater interface {
	UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
		archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error
}
