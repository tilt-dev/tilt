package os

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
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

	err = env.AddValue("os.environ", Environ{})
	if err != nil {
		return err
	}
	err = env.AddBuiltin("os.getenv", getenv)
	if err != nil {
		return err
	}
	err = env.AddBuiltin("os.putenv", putenv)
	if err != nil {
		return err
	}
	err = env.AddBuiltin("os.unsetenv", unsetenv)
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

func getenv(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key starlark.Value
	var defaultVal starlark.Value = starlark.None
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs,
		"key", &key,
		"default?", &defaultVal,
	)
	if err != nil {
		return nil, err
	}

	keyStr, ok := value.AsString(key)
	if !ok {
		return nil, fmt.Errorf("key must be a string, actual: %s", key)
	}

	envVal, found := os.LookupEnv(keyStr)
	if !found {
		return defaultVal, nil
	}

	return starlark.String(envVal), nil
}

func putenv(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, v starlark.String
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs,
		"key", &key,
		"value", &v,
	)
	if err != nil {
		return nil, err
	}

	keyStr, ok := value.AsString(key)
	if !ok {
		return nil, fmt.Errorf("key must be a string, actual: %s", key)
	}

	valueStr, ok := value.AsString(v)
	if !ok {
		return nil, fmt.Errorf("value must be a string, actual: %s", v)
	}

	os.Setenv(keyStr, valueStr)
	return starlark.None, nil
}

func unsetenv(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key starlark.Value
	err := starkit.UnpackArgs(t, fn.Name(), args, kwargs,
		"key", &key,
	)
	if err != nil {
		return nil, err
	}

	keyStr, ok := value.AsString(key)
	if !ok {
		return nil, fmt.Errorf("key must be a string, actual: %s", key)
	}

	os.Unsetenv(keyStr)
	return starlark.None, nil
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

	_, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		err := io.RecordReadPath(t, io.WatchFileOnly, absPath)
		if err != nil {
			return nil, err
		}

		return starlark.Bool(false), nil
	} else if err != nil {
		// Return false on error (e.g., permission denied errors),
		// for consistency with the python version, but don't watch.
		return starlark.Bool(false), nil
	}

	err = io.RecordReadPath(t, io.WatchFileOnly, absPath)
	if err != nil {
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
