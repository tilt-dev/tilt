package hud

type ExitAction struct {
	Err error
}

func (ExitAction) Action() {}

func NewExitAction(err error) ExitAction {
	return ExitAction{err}
}

type DumpEngineStateAction struct {
}

func (DumpEngineStateAction) Action() {}
