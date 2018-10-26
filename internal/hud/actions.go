package hud

type ShowErrorAction struct {
	// 1-based index of resource whose log should be printed
	ResourceNumber int
}

func (ShowErrorAction) Action() {}

func NewShowErrorAction(resourceNumber int) ShowErrorAction {
	return ShowErrorAction{resourceNumber}
}

type ExitAction struct{}

func (ExitAction) Action() {}
