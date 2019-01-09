package dockercompose

import (
	"time"
)

// Three hacky states just for now to get something into the hud.
const (
	StatusDown   = "Down"
	StatusInProg = "In Progress"
	StatusUp     = "OK"
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

type State struct {
	Status     string
	CurrentLog []byte
	StartTime  time.Time
}

func (State) ResourceState() {}

func (s State) Log() string {
	return string(s.CurrentLog)
}

func (s State) WithCurrentLog(b []byte) State {
	s.CurrentLog = b
	return s
}

func (s State) WithStatus(status string) State {
	s.Status = status
	return s
}

func (s State) WithStartTime(time time.Time) State {
	s.StartTime = time
	return s
}
