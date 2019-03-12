package dockercompose

import (
	"time"

	"github.com/windmilleng/tilt/internal/model"

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
	CurrentLog  model.Log
	StartTime   time.Time
	IsStopping  bool
}

func (State) ResourceState() {}

func (s State) Log() string {
	return s.CurrentLog.String()
}

func (s State) WithCurrentLog(l model.Log) State {
	s.CurrentLog = l
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

func (s State) WithStopping(stopping bool) State {
	s.IsStopping = stopping
	return s
}
