package containerupdate

import (
	"context"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type NoopUpdater struct{}

var _ ContainerUpdater = NoopUpdater{}

func NewNoopUpdater() ContainerUpdater {
	return NoopUpdater{}
}

func (cu NoopUpdater) SupportsSpecs(specs []model.TargetSpec) error {
	return fmt.Errorf("NoopUpdater.SupportsSpecs should never be called; please contact Tilt support")
}

func (cu NoopUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	return fmt.Errorf("NoopUpdater.SupportsSpecs should never be called; please contact Tilt support")
}
