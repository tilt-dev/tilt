package tilt

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

// starlark builtins for introspection about Tilt itself

type TiltSubcommand string

type Extension struct {
	tiltSubCommand TiltSubcommand
}

var _ starkit.Extension = Extension{}

func NewExtension(tiltSubCommand TiltSubcommand) Extension {
	return Extension{tiltSubCommand: tiltSubCommand}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddValue("tilt.sub_command", starlark.String(e.tiltSubCommand))
}
