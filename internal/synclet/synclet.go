package synclet

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/logger"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
)

const Port = 23551

type Synclet struct {
	dcli docker.DockerClient
}

func NewSynclet(dcli docker.DockerClient) *Synclet {
	return &Synclet{dcli: dcli}
}

func (s Synclet) writeFiles(ctx context.Context, containerId container.ID, tarArchive []byte) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-writeFiles")
	defer span.Finish()

	if tarArchive == nil {
		return nil
	}

	return s.dcli.CopyToContainerRoot(ctx, containerId.String(), bytes.NewBuffer(tarArchive))
}

func (s Synclet) rmFiles(ctx context.Context, containerId container.ID, filesToDelete []string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-rmFiles")
	defer span.Finish()

	if len(filesToDelete) == 0 {
		return nil
	}

	cmd := model.Cmd{Argv: append([]string{"rm", "-rf"}, filesToDelete...)}

	out := bytes.NewBuffer(nil)
	err := s.dcli.ExecInContainer(ctx, containerId, cmd, out)
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
		err := s.dcli.ExecInContainer(ctx, containerId, c, l.Writer(logger.InfoLvl))
		if err != nil {
			exitError, isExitError := err.(docker.ExitError)
			if isExitError {
				return build.UserBuildFailure{ExitCode: exitError.ExitCode}
			}
			return err
		}
	}
	return nil
}

func (s Synclet) restartContainer(ctx context.Context, containerId container.ID) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-restartContainer")
	defer span.Finish()

	return s.dcli.ContainerRestartNoWait(ctx, containerId.String())
}

func (s Synclet) UpdateContainer(
	ctx context.Context,
	containerId container.ID,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-UpdateContainer")
	defer span.Finish()

	err := s.rmFiles(ctx, containerId, filesToDelete)
	if err != nil {
		return fmt.Errorf("error removing files while updating container %s: %v",
			containerId.ShortStr(), err)
	}

	err = s.writeFiles(ctx, containerId, tarArchive)
	if err != nil {
		return fmt.Errorf("error writing files while updating container %s: %v",
			containerId.ShortStr(), err)
	}

	err = s.execCmds(ctx, containerId, commands)
	if err != nil {
		return fmt.Errorf("error exec'ing commands while updating container %s: %v",
			containerId.ShortStr(), err)
	}

	err = s.restartContainer(ctx, containerId)
	if err != nil {
		return fmt.Errorf("error restarting container %s: %v",
			containerId.ShortStr(), err)
	}

	return nil
}
