package dockercompose

import "reflect"

// Three hacky states just for now to get something into the hud.
const (
	StatusDown   = "down"
	StatusInProg = "in progress"
	StatusUp     = "up"
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

type State struct {
	Status     string
	CurrentLog []byte
}

func (State) ResourceState() {}
func (s State) Empty() bool  { return reflect.DeepEqual(s, State{}) }

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
