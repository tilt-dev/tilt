package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/model"
)

type FakeContainerUpdater struct {
	UpdateErrs []error

	Calls []UpdateContainerCall
}

type UpdateContainerCall struct {
	ContainerInfo liveupdates.Container
	Archive       io.Reader
	ToDelete      []string
	Cmds          []model.Cmd
	HotReload     bool
}

func (cu *FakeContainerUpdater) SetUpdateErr(err error) {
	cu.UpdateErrs = []error{err}
}

func (cu *FakeContainerUpdater) UpdateContainer(ctx context.Context, cInfo liveupdates.Container,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {

	var archive bytes.Buffer
	if _, err := io.Copy(&archive, archiveToCopy); err != nil {
		return fmt.Errorf("FakeContainerUpdater failed to read archive: %v", err)
	}
	cu.Calls = append(cu.Calls, UpdateContainerCall{
		ContainerInfo: cInfo,
		Archive:       &archive,
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
