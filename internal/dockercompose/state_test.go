package dockercompose

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestRuntimeStatus_NoHealthCheck(t *testing.T) {
	// Container running without health check should be OK
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running: true,
			Status:  ContainerStatusRunning,
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusOK, s.RuntimeStatus())
}

func TestRuntimeStatus_HealthCheckStarting(t *testing.T) {
	// Container running but health check still starting should be Pending
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running:      true,
			Status:       ContainerStatusRunning,
			HealthStatus: "starting",
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusPending, s.RuntimeStatus())
}

func TestRuntimeStatus_HealthCheckHealthy(t *testing.T) {
	// Container running and health check healthy should be OK
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running:      true,
			Status:       ContainerStatusRunning,
			HealthStatus: "healthy",
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusOK, s.RuntimeStatus())
}

func TestRuntimeStatus_HealthCheckUnhealthy(t *testing.T) {
	// Container running but health check unhealthy should be Error
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running:      true,
			Status:       ContainerStatusRunning,
			HealthStatus: "unhealthy",
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusError, s.RuntimeStatus())
}

func TestRuntimeStatus_ContainerError(t *testing.T) {
	// Container with error should be Error regardless of health
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running:      false,
			Status:       ContainerStatusExited,
			Error:        "some error",
			HealthStatus: "healthy",
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusError, s.RuntimeStatus())
}

func TestRuntimeStatus_NonZeroExitCode(t *testing.T) {
	// Container with non-zero exit code should be Error
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running:  false,
			Status:   ContainerStatusExited,
			ExitCode: 1,
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusError, s.RuntimeStatus())
}

func TestRuntimeStatus_ExitedSuccessfully(t *testing.T) {
	// Container exited with code 0 and no health check should be OK
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Running:  false,
			Status:   ContainerStatusExited,
			ExitCode: 0,
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusOK, s.RuntimeStatus())
}

func TestRuntimeStatus_EmptyStatus(t *testing.T) {
	// Container with empty status should be Unknown
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Status: "",
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusUnknown, s.RuntimeStatus())
}

func TestRuntimeStatus_CreatedStatus(t *testing.T) {
	// Container in created state should be Pending
	s := State{
		ContainerState: v1alpha1.DockerContainerState{
			Status: ContainerStatusCreated,
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusPending, s.RuntimeStatus())
}

func TestRuntimeStatusError_UnhealthyContainer(t *testing.T) {
	// RuntimeStatusError should return appropriate message for unhealthy container
	s := State{
		ContainerID: "abc123",
		ContainerState: v1alpha1.DockerContainerState{
			Running:      true,
			Status:       ContainerStatusRunning,
			HealthStatus: "unhealthy",
		},
	}
	err := s.RuntimeStatusError()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "health check failed")
	assert.Contains(t, err.Error(), "abc123")
}
