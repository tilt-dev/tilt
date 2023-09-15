package docker

import (
	"fmt"
	"io"

	"github.com/distribution/reference"
	mobycontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

// RunConfig defines the container to create and start.
type RunConfig struct {
	// Image to execute.
	//
	// If Pull is true, this must be a reference.Named.
	Image reference.Reference
	// Pull will ensure the image exists locally before running.
	//
	// If an image will only be used once, this is a convenience to avoid calling ImagePull first.
	// If an image will be used multiple times (across containers), prefer explicitly calling ImagePull
	// to avoid the overhead of calling the registry API to check if the image is up-to-date every time.
	Pull bool
	// ContainerName is a unique name for the container. If not specified, Docker will generate a random name.
	ContainerName string
	// Stdout from the container will be written here if non-nil.
	//
	// Errors copying the container output are logged but not propagated.
	Stdout io.Writer
	// Stderr from the container will be written here if non-nil.
	//
	// Errors copying the container output are logged but not propagated.
	Stderr io.Writer
	// Cmd to run when starting the container.
	Cmd []string
	// Mounts to attach to the container.
	Mounts []mount.Mount
}

// RunResult contains information about a container execution.
type RunResult struct {
	ContainerID string

	logsErrCh    <-chan error
	statusRespCh <-chan mobycontainer.WaitResponse
	statusErrCh  <-chan error
	tearDown     func(containerID string) error
}

// Wait blocks until stdout and stderr have been fully consumed (if writers were passed via RunConfig) and the
// container has exited. If there is any error consuming stdout/stderr or monitoring the container execution, an
// error will be returned.
func (r *RunResult) Wait() (int64, error) {
	select {
	case err := <-r.statusErrCh:
		return -1, err
	case statusResp := <-r.statusRespCh:
		if statusResp.Error != nil {
			return -1, fmt.Errorf("error waiting on container: %s", statusResp.Error.Message)
		}
		logsErr := <-r.logsErrCh
		if logsErr != nil {
			// error is
			return statusResp.StatusCode, fmt.Errorf("error reading container logs: %w", logsErr)
		}
		return statusResp.StatusCode, nil
	}
}

// Close removes the container (forcibly if it's still running).
func (r *RunResult) Close() error {
	if r.tearDown == nil {
		return nil
	}
	err := r.tearDown(r.ContainerID)
	if err != nil {
		return err
	}
	r.tearDown = nil
	return nil
}
