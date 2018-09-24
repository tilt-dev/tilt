package synclet

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/opentracing/opentracing-go"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
)

const Port = 23551

type Synclet struct {
	dcli docker.DockerClient
	cr   *build.ContainerResolver
}

func NewSynclet(dcli docker.DockerClient, cr *build.ContainerResolver) *Synclet {
	return &Synclet{dcli: dcli, cr: cr}
}

func (s Synclet) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (k8s.ContainerID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-ContainerIDForPod")
	defer span.Finish()

	return s.cr.ContainerIDForPod(ctx, podID, imageID)
}

func (s Synclet) writeFiles(ctx context.Context, containerId k8s.ContainerID, tarArchive []byte) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-writeFiles")
	defer span.Finish()

	if tarArchive == nil {
		return nil
	}

	return s.dcli.CopyToContainerRoot(ctx, containerId.String(), bytes.NewBuffer(tarArchive))
}

func (s Synclet) rmFiles(ctx context.Context, containerId k8s.ContainerID, filesToDelete []string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-rmFiles")
	defer span.Finish()

	if len(filesToDelete) == 0 {
		return nil
	}

	cmd := model.Cmd{Argv: append([]string{"rm", "-rf"}, filesToDelete...)}

	out := bytes.NewBuffer(nil)
	err := s.dcli.ExecInContainer(ctx, containerId, cmd, out)
	if err != nil {
		if docker.IsExitError(err) {
			return fmt.Errorf("Error deleting files: %s", out.String())
		}
		return fmt.Errorf("Error deleting files: %v", err)
	}
	return nil
}

func (s Synclet) execCmds(ctx context.Context, containerId k8s.ContainerID, cmds []model.Cmd) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-execCommands")
	defer span.Finish()

	for i, c := range cmds {
		// TODO: instrument this
		log.Printf("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		err := s.dcli.ExecInContainer(ctx, containerId, c, output.Get(ctx).Writer())
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

func (s Synclet) restartContainer(ctx context.Context, containerId k8s.ContainerID) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Synclet-restartContainer")
	defer span.Finish()

	return s.dcli.ContainerRestartNoWait(ctx, containerId.String())
}

func (s Synclet) UpdateContainer(
	ctx context.Context,
	containerId k8s.ContainerID,
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
