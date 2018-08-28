package build

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/distribution/reference"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/windmilleng/tilt/internal/model"
)

type containerID string

func (cID containerID) String() string   { return string(cID) }
func (cID containerID) ShortStr() string { return string(cID)[:10] }

type execID string

// Get a container config to run a container with a given command instead of
// the existing entrypoint. If cmd is nil, we run nothing.
func containerConfigRunCmd(imgRef reference.NamedTagged, cmd model.Cmd) *container.Config {
	config := containerConfig(imgRef)

	// In Docker, both the Entrypoint and the Cmd are used to determine what
	// process the container runtime uses, where Entrypoint takes precedence over
	// command. We set both here to ensure that we don't get weird results due
	// to inheritance.
	//
	// If cmd is nil, we use a fake cmd that does nothing.
	//
	// https://github.com/opencontainers/image-spec/blob/master/config.md#properties
	if cmd.Empty() {
		config.Cmd = model.ToShellCmd("# NOTE(nick): a fake cmd").Argv
	} else {
		config.Cmd = cmd.Argv
	}
	config.Entrypoint = []string{}
	return config
}

// Get a container config to run a container as-is.
func containerConfig(imgRef reference.NamedTagged) *container.Config {
	return &container.Config{Image: imgRef.String()}
}

func execInContainer(ctx context.Context, dcli DockerClient, cID containerID, cmd model.Cmd) error {
	created, err := dcli.ContainerExecCreate(ctx, string(cID), types.ExecConfig{
		Cmd: cmd.Argv,
	})
	if err != nil {
		return err
	}

	execID := execID(created.ID)
	if execID == "" {
		return fmt.Errorf("execInContainer: failed to create")
	}

	attached, err := dcli.ContainerExecAttach(ctx, string(execID), types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("execInContainer#attach: %v", err)
	}
	defer attached.Close()

	// TODO(nick): feed this reader into the logger
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, attached.Reader)
	if err != nil {
		return fmt.Errorf("execInContainer#copy: %v", err)
	}

	for true {
		inspected, err := dcli.ContainerExecInspect(ctx, string(execID))
		if err != nil {
			return fmt.Errorf("execInContainer#inspect: %v", err)
		}

		if inspected.Running {
			continue
		}

		status := inspected.ExitCode
		if status != 0 {
			return fmt.Errorf("Failed with exit code %d. Output:\n%s", status, buf.String())
		}
		return nil
	}
	return nil
}
