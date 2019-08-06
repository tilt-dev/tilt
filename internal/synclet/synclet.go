package synclet

import (
	"bytes"
	"context"
	"log"
	"strings"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/logger"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
)

const Port = 23551

type Synclet struct {
	dCli docker.Client
}

func NewSynclet(dCli docker.Client) *Synclet {
	return &Synclet{dCli: dCli}
}

func (s Synclet) writeFiles(ctx context.Context, containerId container.ID, tarArchive []byte) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-writeFiles")
	defer span.Finish()

	if tarArchive == nil {
		return nil
	}

	return s.dCli.CopyToContainerRoot(ctx, containerId.String(), bytes.NewBuffer(tarArchive))
}

func (s Synclet) rmFiles(ctx context.Context, containerId container.ID, filesToDelete []string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-rmFiles")
	defer span.Finish()

	if len(filesToDelete) == 0 {
		return nil
	}

	cmd := model.Cmd{Argv: append([]string{"rm", "-rf"}, filesToDelete...)}

	out := bytes.NewBuffer(nil)
	err := s.dCli.ExecInContainer(ctx, containerId, cmd, out)
	if err != nil {
		dockerExitErr, ok := err.(docker.ExitError)
		if ok {
			return errors.Wrapf(err, "Error deleting files. exit code %d, output '%s'", dockerExitErr.ExitCode, out.String())
		}
		return errors.Wrap(err, "Error deleting files")
	}
	return nil
}

func (s Synclet) execCmds(ctx context.Context, containerId container.ID, cmds []model.Cmd) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-execCommands")
	defer span.Finish()

	for i, c := range cmds {
		// TODO: instrument this
		log.Printf("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		// TODO(matt) - plumb PipelineState through
		l := logger.Get(ctx)
		err := s.dCli.ExecInContainer(ctx, containerId, c, l.Writer(logger.InfoLvl))
		if err != nil {
			return build.WrapContainerExecError(err, containerId, c)
		}
	}
	return nil
}

func (s Synclet) restartContainer(ctx context.Context, containerId container.ID) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-restartContainer")
	defer span.Finish()

	return s.dCli.ContainerRestartNoWait(ctx, containerId.String())
}

func (s Synclet) UpdateContainer(
	ctx context.Context,
	containerId container.ID,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd,
	hotReload bool) error {

	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-UpdateContainer")
	defer span.Finish()

	err := s.rmFiles(ctx, containerId, filesToDelete)
	if err != nil {
		return errors.Wrapf(err, "error removing files while updating container %s",
			containerId.ShortStr())
	}

	err = s.writeFiles(ctx, containerId, tarArchive)
	if err != nil {
		return errors.Wrapf(err, "error writing files while updating container %s",
			containerId.ShortStr())
	}

	err = s.execCmds(ctx, containerId, commands)
	if err != nil {
		return errors.Wrapf(err, "error exec'ing commands while updating container %s",
			containerId.ShortStr())
	}

	if !hotReload {
		err = s.restartContainer(ctx, containerId)
		if err != nil {
			return errors.Wrapf(err, "error restarting container %s",
				containerId.ShortStr())
		}
	}

	return nil
}
