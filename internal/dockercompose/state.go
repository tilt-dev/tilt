package dockercompose

import (
	"fmt"
	"time"

	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/container"
)

type Status string

// Three hacky states just for now to get something into the hud.
const (
	StatusDown   = Status("Down")
	StatusInProg = Status("In Progress")
	StatusUp     = Status("OK")
	StatusCrash  = Status("Crash")
)

var containerActionToStatus = map[Action]Status{
	ActionCreate:  StatusInProg,
	ActionDie:     StatusDown,
	ActionKill:    StatusDown,
	ActionRename:  StatusInProg,
	ActionRestart: StatusUp, // ??
	ActionStart:   StatusUp,
	ActionStop:    StatusDown,
	ActionUpdate:  StatusUp, // ??
}

func (evt Event) GuessStatus() Status {
	if evt.Type != TypeContainer {
		return ""
	}
	return containerActionToStatus[evt.Action]
}

func (evt Event) IsStartupEvent() bool {
	if evt.Type != TypeContainer {
		return false
	}
	return evt.Action == ActionStart || evt.Action == ActionRestart || evt.Action == ActionUpdate
}

func (evt Event) IsStopEvent() bool {
	if evt.Type != TypeContainer {
		return false
	}

	return evt.Action == ActionKill
}

type State struct {
	Status      Status
	ContainerID container.ID

	StartTime     time.Time
	IsStopping    bool
	LastReadyTime time.Time

	SpanID model.LogSpanID
}

func (State) RuntimeState() {}

func (s State) RuntimeStatus() model.RuntimeStatus {
	runtimeStatus, ok := runtimeStatusMap[string(s.Status)]
	if !ok {
		return model.RuntimeStatusError
	}
	return runtimeStatus
}

func (s State) RuntimeStatusError() error {
	status := s.RuntimeStatus()
	if status != model.RuntimeStatusError {
		return nil
	}
	return fmt.Errorf("Container %s in error state: %s", s.ContainerID, s.Status)
}

func (s State) WithSpanID(spanID model.LogSpanID) State {
	s.SpanID = spanID
	return s
}

func (s State) WithStatus(status Status) State {
	s.Status = status
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

func (s State) WithStopping(stopping bool) State {
	s.IsStopping = stopping
	return s
}

func (s State) HasEverBeenReadyOrSucceeded() bool {
	return !s.LastReadyTime.IsZero()
}

var runtimeStatusMap = map[string]model.RuntimeStatus{
	string(StatusInProg): model.RuntimeStatusPending,
	string(StatusUp):     model.RuntimeStatusOK,
	string(StatusDown):   model.RuntimeStatusError,
	string(StatusCrash):  model.RuntimeStatusError,

	// If the runtime status hasn't shown up yet, we assume it's pending.
	"": model.RuntimeStatusPending,
}
