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

type HeapProfileAction struct{}

func (HeapProfileAction) Action() {}

type SetLogTimestampsAction struct {
	Value bool
}

func (SetLogTimestampsAction) Action() {}

type DumpEngineStateAction struct {
}

func (DumpEngineStateAction) Action() {}
