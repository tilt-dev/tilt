package local

import "github.com/tilt-dev/tilt/internal/store"

type CmdCreateAction struct {
	Cmd *Cmd
}

func NewCmdCreateAction(cmd *Cmd) CmdCreateAction {
	return CmdCreateAction{Cmd: cmd.DeepCopy()}
}

var _ store.Summarizer = CmdCreateAction{}

func (CmdCreateAction) Action() {}

func (a CmdCreateAction) Summarize(s *store.ChangeSummary) {
	if s.CmdSpecs == nil {
		s.CmdSpecs = make(map[string]bool)
	}
	s.CmdSpecs[a.Cmd.Name] = true
}

type CmdUpdateStatusAction struct {
	Cmd *Cmd
}

func NewCmdUpdateStatusAction(cmd *Cmd) CmdUpdateStatusAction {
	return CmdUpdateStatusAction{Cmd: cmd.DeepCopy()}
}

func (CmdUpdateStatusAction) Action() {}

type CmdDeleteAction struct {
	Name string
}

func (CmdDeleteAction) Action() {}

func (a CmdDeleteAction) Summarize(s *store.ChangeSummary) {
	if s.CmdSpecs == nil {
		s.CmdSpecs = make(map[string]bool)
	}
	s.CmdSpecs[a.Name] = true
}
