package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ContainerUpdater interface {
	UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
		archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error
	CanUpdateSpecs(specs []model.TargetSpec) (canUpd bool, msg string, silent bool)
}
