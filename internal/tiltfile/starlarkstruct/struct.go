package starlarkstruct

import (
	"go.starlark.net/starlarkstruct"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) OnStart(env *starkit.Environment) error {
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
