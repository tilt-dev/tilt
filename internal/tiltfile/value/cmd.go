package value

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

const typeCmd = "cmd"

type CmdPlugin struct{}

func NewCmdPlugin() CmdPlugin {
	return CmdPlugin{}
}

var _ starkit.Plugin = CmdPlugin{}

func (p CmdPlugin) OnStart(env *starkit.Environment) error {
	if err := env.AddBuiltin("cmd", p.cmd); err != nil {
		return fmt.Errorf("could not add cmd builtin: %v", err)
	}
	return nil
}

func (p CmdPlugin) cmd(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple,
	kwargs []starlark.Tuple) (starlark.Value, error) {
	var argsVal, argsBatVal starlark.Value
	// TODO(milas): should we also except ['FOO=bar'] style list?
	var env StringStringMap
	var dir starlark.Value

	err := starkit.UnpackArgs(
		thread, fn.Name(), args, kwargs,
		"args", &argsVal,
		"env?", &env,
		"dir?", &dir,
		"args_bat?", &argsBatVal,
	)
	if err != nil {
		return nil, err
	}

	cmd, err := ValueGroupToCmdHelper(thread, argsVal, argsBatVal, dir, env)
	if err != nil {
		return nil, err
	}

	return Cmd{
		Struct: starlarkstruct.FromKeywords(
			starlark.String(typeCmd), []starlark.Tuple{
				{starlark.String("args"), StringSliceToList(cmd.Argv)},
				{starlark.String("env"), StringSliceToList(cmd.Env)},
				{starlark.String("dir"), starlark.String(cmd.Dir)},
			},
		),
		cmd: cmd,
	}, nil
}

type Cmd struct {
	*starlarkstruct.Struct
	cmd model.Cmd
}

func (c Cmd) ToModelCmd() model.Cmd {
	return c.cmd
}
