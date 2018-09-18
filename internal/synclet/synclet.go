package synclet

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
)

const Port = 23551

type Synclet struct {
	dcli docker.DockerClient
}

func NewSynclet(dcli docker.DockerClient) *Synclet {
	return &Synclet{dcli: dcli}
}

const pauseCmd = "/pause"

// TODO(matt) dedupe with ContainerUpdater - https://app.clubhouse.io/windmill/story/227/de-dupe-containeridforpod
func (s Synclet) GetContainerIdForPod(ctx context.Context, podId k8s.PodID) (string, error) {
	a := filters.NewArgs()
	a.Add("name", podId.String())
	listOpts := types.ContainerListOptions{Filters: a}
	containers, err := s.dcli.ContainerList(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("getting containers: %v", err)
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("no containers found with name %s", podId)
	}

	// We expect there to be one real match and one spurious match -- a container running
	// "/pause" (see: https://www.ianlewis.org/en/almighty-pause-container); filter it out
	if len(containers) > 2 {
		var ids []string
		for _, c := range containers {
			ids = append(ids, c.ID[:10])
		}
		return "", fmt.Errorf("too many matching containers (%v)", ids)
	}

	for _, c := range containers {
		// TODO(maia): more robust check here (what if user is running a container with "/pause" command?!)
		if c.Command != pauseCmd {
			return c.ID, nil
		}
	}

	// What?? No actual matches??!
	return "", fmt.Errorf("no real containers -- all were '/pause' containers")
}

func (s Synclet) writeFiles(ctx context.Context, containerId k8s.ContainerID, tarArchive []byte) error {
	if tarArchive == nil {
		return nil
	}

	return s.dcli.CopyToContainerRoot(ctx, containerId.String(), bytes.NewBuffer(tarArchive))
}

func (s Synclet) rmFiles(ctx context.Context, containerId k8s.ContainerID, filesToDelete []string) error {
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
	return s.dcli.ContainerRestartNoWait(ctx, containerId.String())
}

func (s Synclet) UpdateContainer(
	ctx context.Context,
	containerId k8s.ContainerID,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

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
