package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type FakeContainerUpdater struct {
	UpdateErr error

	Calls []UpdateContainerCall
}

type UpdateContainerCall struct {
	ContainerInfo store.ContainerInfo
	Archive       io.Reader
	ToDelete      []string
	Cmds          []model.Cmd
	HotReload     bool
}

func (cu *FakeContainerUpdater) UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	cu.Calls = append(cu.Calls, UpdateContainerCall{
		ContainerInfo: cInfo,
		Archive:       archiveToCopy,
		ToDelete:      filesToDelete,
		Cmds:          cmds,
		HotReload:     hotReload,
	})

	err := cu.UpdateErr
	cu.UpdateErr = nil
	return err
}
