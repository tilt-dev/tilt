package include

import (
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

// Implements the include() built-in.
//
// The main difference is that include() doesn't bind any arguments into the
// global scope, whereas load() forces you to bind at least one argument into the global
// scope (i.e., you can't load() a Tilfile for its side-effects).
type IncludeFn struct {
}

func (IncludeFn) OnStart(e *starkit.Environment) error {
	return e.AddBuiltin("include", include)
}

func include(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var p string
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs, "path", &p)
	if err != nil {
		return nil, err
	}

	_, err = t.Load(t, p)
	return starlark.None, err
}
