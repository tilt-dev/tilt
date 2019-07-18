package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type FakeContainerUpdater struct {
	UpdateErrToThrow error

	Calls []UpdateContainerCall
}

type UpdateContainerCall struct {
	DeployInfo store.DeployInfo
	Archive    io.Reader
	ToDelete   []string
	Cmds       []model.Cmd
	HotReload  bool
}

func (cu *FakeContainerUpdater) CanUpdateSpecs(specs []model.TargetSpec) (canUpd bool, msg string, silent bool) {
	// TODO(maia): implement
	return true, "", false
}

func (cu *FakeContainerUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	cu.Calls = append(cu.Calls, UpdateContainerCall{
		DeployInfo: deployInfo,
		Archive:    archiveToCopy,
		ToDelete:   filesToDelete,
		Cmds:       cmds,
		HotReload:  hotReload,
	})

	var err error
	if cu.UpdateErrToThrow != nil {
		err = cu.UpdateErrToThrow
		cu.UpdateErrToThrow = nil
	}
	return err
}
