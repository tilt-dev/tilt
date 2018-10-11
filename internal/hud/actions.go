package hud

type ReplayBuildLogAction struct {
	// 1-based index of resource whose log should be printed
	ResourceNumber int
}

func (ReplayBuildLogAction) Action() {}

func NewReplayBuildLogAction(resourceNumber int) ReplayBuildLogAction {
	return ReplayBuildLogAction{resourceNumber}
}
