package os

import (
	"os"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

// The starlark OS module.
// Modeled after Bazel's repository_os
// https://docs.bazel.build/versions/master/skylark/lib/repository_os.html
// and Python's OS module
// https://docs.python.org/3/library/os.html
type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	err := env.AddBuiltin("os.getcwd", cwd)
	if err != nil {
		return err
	}

	environValue, err := environ()
	if err != nil {
		return err
	}
	return env.AddValue("os.environ", environValue)
}

func environ() (starlark.Value, error) {
	env := os.Environ()
	result := starlark.NewDict(len(env))
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		err := result.SetKey(starlark.String(pair[0]), starlark.String(pair[1]))
		if err != nil {
			return nil, err
		}
	}
	result.Freeze()
	return result, nil
}

// Fetch the working directory of current Tiltfile execution.
// All built-ins will be executed relative to this directory (e.g., local(), docker_build(), etc)
// Intended to mirror the API of Python's getcwd
// https://docs.python.org/3/library/os.html#os.getcwd
func cwd(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}

	dir := starkit.AbsWorkingDir(t)
	return starlark.String(dir), nil
}
