package print

import (
	"errors"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Additional functions for print output.
type Plugin struct{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (Plugin) OnStart(env *starkit.Environment) error {
	err := env.AddBuiltin("warn", warn)
	if err != nil {
		return err
	}
	err = env.AddBuiltin("fail", fail)
	if err != nil {
		return err
	}
	err = env.AddBuiltin("exit", exit)
	if err != nil {
		return err
	}
	return nil
}

func fail(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "msg", &msg)
	if err != nil {
		return nil, err
	}

	return nil, errors.New(msg)
}

func warn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "msg", &msg)
	if err != nil {
		return nil, err
	}

	ctx, err := starkit.ContextFromThread(thread)
	if err != nil {
		return nil, err
	}

	logger.Get(ctx).Warnf("%s", msg)

	return starlark.None, nil
}

func exit(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var codeVal starlark.Value
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "code?", &codeVal)
	if err != nil {
		return nil, err
	}

	ctx, err := starkit.ContextFromThread(thread)
	if err != nil {
		return nil, err
	}

	if codeVal != nil && codeVal != starlark.None {
		code := codeVal.String()
		if code != "" {
			logger.Get(ctx).Infof("%s", code)
		}
	}

	return starlark.None, starkit.ErrStopExecution
}
