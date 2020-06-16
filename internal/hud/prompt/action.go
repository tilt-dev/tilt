package prompt

import "github.com/tilt-dev/tilt/internal/store"

type SwitchTerminalModeAction struct {
	Mode store.TerminalMode
}

func (SwitchTerminalModeAction) Action() {}

var _ store.Action = SwitchTerminalModeAction{}
