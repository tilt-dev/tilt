package exit

type Action struct {
	ExitSignal bool
	ExitError  error
}

func (Action) Action() {}
