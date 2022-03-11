package eval

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

type Plugin struct {
}

func NewPlugin() Plugin {
	return Plugin{}
}

type evaluator struct {
	env *starkit.Environment
}

func (e Plugin) OnStart(env *starkit.Environment) error {
	ev := &evaluator{env: env}
	err := env.AddBuiltin("eval", ev.eval)
	if err != nil {
		return err
	}
	return env.AddBuiltin("exec", ev.exec)
}

func (e *evaluator) exec(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	src, env, err := e.prepare(t, fn, args, kwargs)
	if err != nil {
		return nil, err
	}

	globals, err := starlark.ExecFile(t, "__exec__", src, env)
	if err != nil {
		return nil, err
	}

	dict := starlark.NewDict(len(globals))
	for k, v := range globals {
		err = dict.SetKey(starlark.String(k), v)
		if err != nil {
			return nil, err
		}
	}
	return dict, nil
}

func (e *evaluator) eval(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	src, env, err := e.prepare(t, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.Eval(t, "__eval__", src, env)
}

func (e *evaluator) prepare(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (string, starlark.StringDict, error) {
	var code starlark.Value
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs, "code", &code)
	if err != nil {
		return "", nil, err
	}
	var src string
	switch code.(type) {
	case io.Blob:
		src = code.String()
	default:
		var ok bool
		src, ok = starlark.AsString(code)
		if !ok {
			return "", nil, fmt.Errorf("eval: argument %v is not a string", code)
		}
	}
	env := starlark.StringDict{}
	depth := t.CallStackDepth()
	for i := 0; i < depth; i++ {
		frame := t.DebugFrame(i)
		callable := frame.Callable()
		caller, ok := callable.(*starlark.Function)
		if !ok {
			continue
		}
		for k, v := range caller.Globals() {
			if !env.Has(k) {
				env[k] = v
			}
		}
	}

	for k, v := range e.env.Predeclared() {
		if !env.Has(k) {
			env[k] = v
		}
	}

	return src, env, nil
}
