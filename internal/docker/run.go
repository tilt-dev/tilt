package docker

import (
	"fmt"
	"io"

	mobycontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// RunOptions defines the container to create and start.
type RunOptions struct {
	// Pull will ensure the image exists locally before running.
	//
	// If an image will only be used once, this is a convenience to avoid calling ImagePull first.
	// If an image will be used multiple times (across containers), prefer explicitly calling ImagePull
	// to avoid the overhead of calling the registry API to check if the image is up-to-date every time.
	Pull bool
	// ContainerName is a unique name for the container. If not specified, Docker will generate a random name.
	ContainerName string
	// ContainerConfig is the main configuration for the container.
	//
	// The Image field MUST be populated at a minimum.
	ContainerConfig mobycontainer.Config
	// HostConfig is host-level options such as bind mounts (optional).
	HostConfig *mobycontainer.HostConfig
	// NetworkConfig is the network interface configuration for the container (optional).
	NetworkConfig *network.NetworkingConfig
	// Stdout from the container will be written here if non-nil.
	//
	// Errors copying the container output are logged but not propagated.
	Stdout io.Writer
	// Stderr from the container will be written here if non-nil.
	//
	// Errors copying the container output are logged but not propagated.
	Stderr io.Writer
}

// RunResult contains information about a container execution.
type RunResult struct {
	ContainerID string

	logsErrCh    <-chan error
	statusRespCh <-chan mobycontainer.ContainerWaitOKBody
	statusErrCh  <-chan error
}

// Wait blocks until stdout and stderr have been fully consumed (if writers were passed via RunOptions) and the
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
