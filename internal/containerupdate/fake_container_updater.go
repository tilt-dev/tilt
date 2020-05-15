package containerupdate

import (
	"context"
	"io"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type FakeContainerUpdater struct {
	UpdateErrs []error

	Calls []UpdateContainerCall
}

type UpdateContainerCall struct {
	ContainerInfo store.ContainerInfo
	Archive       io.Reader
	ToDelete      []string
	Cmds          []model.Cmd
	HotReload     bool
}

func (cu *FakeContainerUpdater) SetUpdateErr(err error) {
	cu.UpdateErrs = []error{err}
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

	// If we're supposed to throw an error on this call, throw it (and pop from
	// the list of UpdateErrs)
	var err error
	if len(cu.UpdateErrs) > 0 {
		err = cu.UpdateErrs[0]
		cu.UpdateErrs = append([]error{}, cu.UpdateErrs[1:]...)
	}
	return err
}
