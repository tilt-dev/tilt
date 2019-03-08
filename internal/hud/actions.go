package hud

type ExitAction struct {
	Err error
}

func (ExitAction) Action() {}

func NewExitAction(err error) ExitAction {
	return ExitAction{err}
}

type StartProfilingAction struct {
}

func (StartProfilingAction) Action() {}

type StopProfilingAction struct {
}

func (StopProfilingAction) Action() {}

type SetLogTimestampsAction struct {
	Value bool
}

func (SetLogTimestampsAction) Action() {}
