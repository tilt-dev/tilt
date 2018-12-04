package dockercompose

// Three hacky states just for now to get something into the hud.
const (
	StateDown   = "down"
	StateInProg = "in progress"
	StateUp     = "up"
)

var containerActionToState = map[Action]string{
	ActionCreate:  StateInProg,
	ActionDie:     StateDown,
	ActionKill:    StateDown,
	ActionRename:  StateInProg,
	ActionRestart: StateUp, // ??
	ActionStart:   StateUp,
	ActionStop:    StateDown,
	ActionUpdate:  StateUp, // ??
}

func (evt Event) GuessState() (string, bool) {
	if evt.Type != TypeContainer {
		return "", false
	}
	state, ok := containerActionToState[evt.Action]
	return state, ok
}

type Info struct {
	State      string
	CurrentLog []byte
}

func (i Info) Log() string {
	return string(i.CurrentLog)
}
