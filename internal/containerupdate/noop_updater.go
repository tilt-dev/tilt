package containerupdate

import (
	"context"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ExplodingContainerUpdater struct{}

var _ ContainerUpdater = ExplodingContainerUpdater{}

func NewExplodingContainerUpdater() ContainerUpdater {
	return ExplodingContainerUpdater{}
}

func (cu ExplodingContainerUpdater) SupportsSpecs(specs []model.TargetSpec) (supported bool, msg string) {
	return false, "ExplodingContainerUpdater.SupportsSpecs should never be called; please contact Tilt support"
}

func (cu ExplodingContainerUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	return fmt.Errorf("ExplodingContainerUpdater.UpdateContainer should never be called; please contact Tilt support")
}
