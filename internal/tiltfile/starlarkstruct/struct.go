package starlarkstruct

import (
	"go.starlark.net/starlarkstruct"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

type Plugin struct {
}

func NewPlugin() Plugin {
	return Plugin{}
}

func (e Plugin) OnStart(env *starkit.Environment) error {
	err := env.AddBuiltin("struct", starlarkstruct.Make)
	if err != nil {
		return err
	}

	err = env.AddBuiltin("module", starlarkstruct.MakeModule)
	if err != nil {
		return err
	}

	return nil
}
