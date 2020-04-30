package os

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
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

	err = addPathBuiltin(env, "os.path.abspath", abspath)
	if err != nil {
		return err
	}
	err = addPathBuiltin(env, "os.path.basename", basename)
	if err != nil {
		return err
	}
	err = addPathBuiltin(env, "os.path.dirname", dirname)
	if err != nil {
		return err
	}
	err = addPathBuiltin(env, "os.path.exists", exists)
	if err != nil {
		return err
	}
	err = env.AddBuiltin("os.path.join", join)
	if err != nil {
		return err
	}
	err = addPathBuiltin(env, "os.path.realpath", realpath)
	if err != nil {
		return err
	}

	environValue, err := environ()
	if err != nil {
		return err
	}
	err = env.AddValue("os.environ", environValue)
	if err != nil {
		return err
	}

	return env.AddValue("os.name", starlark.String(osName()))
}

// For consistency with
// https://docs.python.org/3/library/os.html#os.name
func osName() string {
	if runtime.GOOS == "windows" {
		return "nt"
	}
	return "posix"
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

// Add a function that takes exactly one parameter, a path string.
func addPathBuiltin(env *starkit.Environment, name string,
	f func(t *starlark.Thread, s string) (starlark.Value, error)) error {
	builtin := func(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var path string
		err := starkit.UnpackArgs(t, fn.Name(), args, kwargs,
			"path", &path,
		)
		if err != nil {
			return nil, err
		}
		return f(t, path)
	}
	return env.AddBuiltin(name, starkit.Function(builtin))
}

func abspath(t *starlark.Thread, path string) (starlark.Value, error) {
	return starlark.String(starkit.AbsPath(t, path)), nil
}

func basename(t *starlark.Thread, path string) (starlark.Value, error) {
	return starlark.String(filepath.Base(path)), nil
}

func dirname(t *starlark.Thread, path string) (starlark.Value, error) {
	return starlark.String(filepath.Dir(path)), nil
}

func exists(t *starlark.Thread, path string) (starlark.Value, error) {
	absPath := starkit.AbsPath(t, path)
	err := io.RecordReadFile(t, absPath)
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(absPath)
	if os.IsNotExist(err) {
		return starlark.Bool(false), nil
	} else if err != nil {
		return nil, err
	}
	return starlark.Bool(true), nil
}

func join(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	parts := []string{}
	for i, arg := range args {
		s, ok := starlark.AsString(arg)
		if !ok {
			return nil, fmt.Errorf("os.path.join() only accepts strings. Argument #%d: %s", i, arg)
		}
		parts = append(parts, s)
	}
	return starlark.String(filepath.Join(parts...)), nil
}

func realpath(t *starlark.Thread, path string) (starlark.Value, error) {
	realPath, err := filepath.EvalSymlinks(starkit.AbsPath(t, path))
	if err != nil {
		return nil, err
	}
	return starlark.String(realPath), nil
}
