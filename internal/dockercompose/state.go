package dockercompose

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

func (evt Event) GuessStatus() (string, bool) {
	if evt.Type != TypeContainer {
		return "", false
	}
	state, ok := containerActionToStatus[evt.Action]
	return state, ok
}

type State struct {
	Status     string
	CurrentLog []byte
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
