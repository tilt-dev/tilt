package sys

import (
	"os"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

// The starlark sys module.
// Contains a subset of Python's sys module.
// https://docs.python.org/3/library/sys.html
type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	err := env.AddValue("sys.argv", argv())
	if err != nil {
		return err
	}

	err = env.AddValue("sys.executable", executable())
	if err != nil {
		return err
	}
	return nil
}

// List of commandline arguments that Tilt started with.
func argv() starlark.Value {
	values := []starlark.Value{}
	for _, arg := range os.Args {
		values = append(values, starlark.String(arg))
	}

	list := starlark.NewList(values)
	list.Freeze()
	return list
}

// Full path to the Tilt executable.
func executable() starlark.Value {
	e, err := os.Executable()
	if err != nil {
		return starlark.None
	}
	return starlark.String(e)
}
