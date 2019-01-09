package dockercompose

import (
	"time"
)

// Three hacky states just for now to get something into the hud.
const (
	StatusDown   = "Down"
	StatusInProg = "In Progress"
	StatusUp     = "OK"
	StatusCrash  = "Crash"
)

var containerActionToStatus = map[Action]string{
	ActionCreate:  StatusInProg,
	ActionDie:     StatusDown,
	ActionKill:    StatusDown,
	ActionRename:  StatusInProg,
	ActionRestart: StatusUp, // ??
	ActionStart:   StatusUp,
	ActionStop:    StatusDown,
	ActionUpdate:  StatusUp, // ??
}

func (evt Event) GuessStatus() string {
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
	Status     string
	CurrentLog []byte
	StartTime  time.Time
	IsStopping bool
}

func (State) ResourceState() {}

func (s State) Log() string {
	return string(s.CurrentLog)
}

func (s State) WithCurrentLog(b []byte) State {
	s.CurrentLog = b
	return s
}

// TODO(dmiller): this should take a status type or something to guarantee that it's one of the valid statuses
func (s State) WithStatus(status string) State {
	s.Status = status
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
