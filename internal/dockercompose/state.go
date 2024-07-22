package dockercompose

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Status strings taken from comments on:
// https://godoc.org/github.com/docker/docker/api/types#ContainerState
const ContainerStatusCreated = "created"
const ContainerStatusRunning = "running"
const ContainerStatusPaused = "paused"
const ContainerStatusRestarting = "restarting"
const ContainerStatusRemoving = "removing"
const ContainerStatusExited = "exited"
const ContainerStatusDead = "dead"

// Helper functions for dealing with ContainerState.
const ZeroTime = "0001-01-01T00:00:00Z"

type State struct {
	ContainerState v1alpha1.DockerContainerState
	ContainerID    container.ID
	Ports          []v1alpha1.DockerPortBinding
	LastReadyTime  time.Time

	SpanID model.LogSpanID
}

func (State) RuntimeState() {}

func (s State) RuntimeStatus() v1alpha1.RuntimeStatus {
	if s.ContainerState.Error != "" || s.ContainerState.ExitCode != 0 {
		return v1alpha1.RuntimeStatusError
	}
	if s.ContainerState.Status == ContainerStatusRestarting {
		return v1alpha1.RuntimeStatusPending
	}
	if s.ContainerState.Running ||
		s.ContainerState.Status == ContainerStatusRunning ||
		s.ContainerState.Status == ContainerStatusExited {
		return v1alpha1.RuntimeStatusOK
	}
	if s.ContainerState.Status == "" {
		return v1alpha1.RuntimeStatusUnknown
	}
	return v1alpha1.RuntimeStatusPending
}

func (s State) RuntimeStatusError() error {
	status := s.RuntimeStatus()
	if status != v1alpha1.RuntimeStatusError {
		return nil
	}
	if s.ContainerState.Error != "" {
		return fmt.Errorf("Container %s: %s", s.ContainerID, s.ContainerState.Error)
	}
	if s.ContainerState.ExitCode != 0 {
		return fmt.Errorf("Container %s exited with %d", s.ContainerID, s.ContainerState.ExitCode)
	}
	return fmt.Errorf("Container %s error status: %s", s.ContainerID, s.ContainerState.Status)
}

func (s State) WithContainerState(state v1alpha1.DockerContainerState) State {
	s.ContainerState = state

	if s.RuntimeStatus() == v1alpha1.RuntimeStatusOK {
		s.LastReadyTime = time.Now()
	}

	return s
}

func (s State) WithPorts(ports []v1alpha1.DockerPortBinding) State {
	s.Ports = ports
	return s
}

func (s State) WithSpanID(spanID model.LogSpanID) State {
	s.SpanID = spanID
	return s
}

func (s State) WithContainerID(cID container.ID) State {
	if cID == s.ContainerID {
		return s
	}
	s.ContainerID = cID
	s.ContainerState = v1alpha1.DockerContainerState{}
	return s
}

func (s State) HasEverBeenReadyOrSucceeded() bool {
	return !s.LastReadyTime.IsZero()
}

func populateContainerStatus(state *types.ContainerState) string {
	if state.Health == nil {
		return state.Status
	}
	if state.Health.Status == types.Starting {
		return ContainerStatusRestarting
	}
	return state.Status
}

func populateContainerError(state *types.ContainerState) string {
	if state.Error != "" {
		return state.Error
	}
	if state.Health == nil || state.Health.Status != types.Unhealthy {
		return ""
	}
	result := "container is unhealthy"
	log := state.Health.Log
	if len(log) > 0 {
		last := log[len(log)-1]
		if last != nil && last.Output != "" {
			result = result + ": " + last.Output
		}
	}
	return result
}

// Convert ContainerState into an apiserver-compatible state model.
func ToContainerState(state *types.ContainerState) *v1alpha1.DockerContainerState {
	if state == nil {
		return nil
	}
	var startedAt, finishedAt time.Time
	var err error
	if state.StartedAt != "" && state.StartedAt != ZeroTime {
		startedAt, err = time.Parse(time.RFC3339Nano, state.StartedAt)
		if err != nil {
			startedAt = time.Time{}
		}
	}

	if state.FinishedAt != "" && state.FinishedAt != ZeroTime {
		finishedAt, err = time.Parse(time.RFC3339Nano, state.FinishedAt)
		if err != nil {
			finishedAt = time.Time{}
		}
	}

	return &v1alpha1.DockerContainerState{
		Status:     populateContainerStatus(state),
		Running:    state.Running,
		Error:      populateContainerError(state),
		ExitCode:   int32(state.ExitCode),
		StartedAt:  metav1.NewMicroTime(startedAt),
		FinishedAt: metav1.NewMicroTime(finishedAt),
	}
}

// Convert a full into an apiserver-compatible status model.
func ToServiceStatus(id container.ID, name string, state *types.ContainerState, ports nat.PortMap) v1alpha1.DockerComposeServiceStatus {
	status := v1alpha1.DockerComposeServiceStatus{}
	status.ContainerID = string(id)
	status.ContainerName = name
	status.ContainerState = ToContainerState(state)

	for containerPort, bindings := range ports {
		for _, binding := range bindings {
			p, err := strconv.Atoi(binding.HostPort)
			if err != nil || p == 0 {
				continue
			}
			status.PortBindings = append(status.PortBindings, v1alpha1.DockerPortBinding{
				ContainerPort: int32(containerPort.Int()),
				HostIP:        binding.HostIP,
				HostPort:      int32(p),
			})
		}
	}

	// `ports` is a map, so make sure the ports come out in a deterministic order.
	sort.Slice(status.PortBindings, func(i, j int) bool {
		pi := status.PortBindings[i]
		pj := status.PortBindings[j]
		if pi.HostPort < pj.HostPort {
			return true
		}
		if pi.HostPort > pj.HostPort {
			return false
		}
		return pi.HostIP < pj.HostIP
	})
	return status
}
