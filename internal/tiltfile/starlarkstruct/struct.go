package starlarkstruct

import (
	"go.starlark.net/starlarkstruct"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("struct", starlarkstruct.Make)
}
