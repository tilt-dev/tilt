package dockercompose

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/container"
)

func (evt Event) IsStartupEvent() bool {
	if evt.Type != TypeContainer {
		return false
	}
	return evt.Action == ActionStart || evt.Action == ActionRestart || evt.Action == ActionUpdate
}

type State struct {
	ContainerState types.ContainerState
	ContainerID    container.ID

	// TODO(nick): It might make since to get rid of StartTime
	// and parse it out of the ContainerState.StartedAt string,
	// though we would have to decide how to treat containers
	// started before Tilt start.
	StartTime time.Time

	LastReadyTime time.Time

	SpanID model.LogSpanID
}

func (State) RuntimeState() {}

func (s State) RuntimeStatus() model.RuntimeStatus {
	if s.ContainerState.Error != "" || s.ContainerState.ExitCode != 0 {
		return model.RuntimeStatusError
	}
	if s.ContainerState.Running ||
		// Status strings taken from comments on:
		// https://godoc.org/github.com/docker/docker/api/types#ContainerState
		s.ContainerState.Status == "running" ||
		s.ContainerState.Status == "exited" {
		return model.RuntimeStatusOK
	}
	if s.ContainerState.Status == "" {
		return model.RuntimeStatusUnknown
	}
	return model.RuntimeStatusPending
}

func (s State) RuntimeStatusError() error {
	status := s.RuntimeStatus()
	if status != model.RuntimeStatusError {
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

func (s State) WithContainerState(state types.ContainerState) State {
	s.ContainerState = state
	return s
}

func (s State) WithSpanID(spanID model.LogSpanID) State {
	s.SpanID = spanID
	return s
}

func (s State) WithContainerID(cID container.ID) State {
	s.ContainerID = cID
	return s
}

func (s State) WithStartTime(time time.Time) State {
	s.StartTime = time
	return s
}

func (s State) WithLastReadyTime(time time.Time) State {
	s.LastReadyTime = time
	return s
}

func (s State) HasEverBeenReadyOrSucceeded() bool {
	return !s.LastReadyTime.IsZero()
}
