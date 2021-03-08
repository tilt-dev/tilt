package local

type CmdCreateAction struct {
	Cmd *Cmd
}

func NewCmdCreateAction(cmd *Cmd) CmdCreateAction {
	return CmdCreateAction{Cmd: cmd.DeepCopy()}
}

func (CmdCreateAction) Action() {}

type CmdUpdateAction struct {
	Cmd *Cmd
}

func NewCmdUpdateAction(cmd *Cmd) CmdUpdateAction {
	return CmdUpdateAction{Cmd: cmd.DeepCopy()}
}

func (CmdUpdateAction) Action() {}
