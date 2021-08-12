package loaddynamic

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

// Implements the load_dynamic() built-in.
//
// Most programming languages only support static import - where the module
// being loaded and the local variables being bound must be determinable at
// compile-time (i.e., without executing the code).
//
// Dynamic import tends to be fairly contentious. Here's a good discussion on the topic:
//
// https://github.com/tc39/proposal-dynamic-import
//
// (TC39 - the JavaScript committee - is a generally good resource for
// programming language theory discussion, because there are so many open-source
// implementations of the core language.)
//
// load_dynamic() provides a dynamic import, with semantics similar to nodejs
// require(), where it returns a dictionary of symbols that can be introspected
// on. It does no binding of local variables.
type LoadDynamicFn struct {
}

func NewPlugin() LoadDynamicFn {
	return LoadDynamicFn{}
}

func (LoadDynamicFn) OnStart(e *starkit.Environment) error {
	return e.AddBuiltin("load_dynamic", loadDynamic)
}

func loadDynamic(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var p value.Stringable
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs, "path", &p)
	if err != nil {
		return nil, err
	}

	module, err := t.Load(t, p.Value)
	if err != nil {
		return nil, err
	}

	dict := starlark.NewDict(len(module))
	for key, val := range module {
		err = dict.SetKey(starlark.String(key), val)
		if err != nil {
			return nil, err
		}
	}
	dict.Freeze()
	return dict, err
}
