package starkit

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

// The main entrypoint to starkit.
// Execute a file with a set of starlark extensions.
func ExecFile(path string, extensions ...Extension) error {
	return newEnvironment(extensions...).start(path)
}

const argUnpackerKey = "starkit.ArgUnpacker"

// Unpacks args, using the arg unpacker on the current thread.
func UnpackArgs(t *starlark.Thread, fnName string, args starlark.Tuple, kwargs []starlark.Tuple, pairs ...interface{}) error {
	env, ok := t.Local(argUnpackerKey).(*Environment)
	if !ok {
		return starlark.UnpackArgs(fnName, args, kwargs, pairs...)
	}
	return env.unpackArgs(fnName, args, kwargs, pairs...)
}

// A starlark execution environment.
type Environment struct {
	unpackArgs  ArgUnpacker
	loadCache   map[string]loadCacheEntry
	predeclared starlark.StringDict
	print       func(thread *starlark.Thread, msg string)
	extensions  []Extension
}

func newEnvironment(extensions ...Extension) *Environment {
	return &Environment{
		unpackArgs:  starlark.UnpackArgs,
		loadCache:   make(map[string]loadCacheEntry),
		extensions:  append([]Extension{}, extensions...),
		predeclared: starlark.StringDict{},
	}
}

func (e *Environment) SetArgUnpacker(unpackArgs ArgUnpacker) {
	e.unpackArgs = unpackArgs
}

// Add a builtin to the environment.
//
// All builtins will be wrapped to invoke OnBuiltinCall on every extension.
//
// All builtins should use starkit.UnpackArgs to get instrumentation.
func (e *Environment) AddBuiltin(name string, b func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)) {
	wrapped := starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		for _, ext := range e.extensions {
			onBuiltinCallExt, ok := ext.(OnBuiltinCallExtension)
			if ok {
				onBuiltinCallExt.OnBuiltinCall(name, fn)
			}
		}

		return b(thread, fn, args, kwargs)
	})

	e.predeclared[name] = wrapped
}

func (e *Environment) AddValue(name string, val starlark.Value) {
	e.predeclared[name] = val
}

func (e *Environment) SetPrint(print func(thread *starlark.Thread, msg string)) {
	e.print = print
}

func (e *Environment) start(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, "environment#start")
	}

	for _, ext := range e.extensions {
		ext.OnStart(e)
	}

	t := &starlark.Thread{
		Print: e.print,
		Load:  e.load,
	}
	t.SetLocal(argUnpackerKey, e)

	_, err = e.exec(t, path)
	return err
}

func (e *Environment) load(t *starlark.Thread, path string) (starlark.StringDict, error) {
	return e.exec(t, AbsPath(t, path))
}

func (e *Environment) exec(t *starlark.Thread, path string) (starlark.StringDict, error) {
	// If the path isn't absolute, the loadCache won't work correctly.
	if !filepath.IsAbs(path) {
		return starlark.StringDict{}, fmt.Errorf("internal error: path must be absolute")
	}

	entry := e.loadCache[path]
	status := entry.status
	if status == loadStatusExecuting {
		return starlark.StringDict{}, fmt.Errorf("Circular load: %s", path)
	} else if status == loadStatusDone {
		return entry.exports, entry.err
	}

	e.loadCache[path] = loadCacheEntry{
		status: loadStatusExecuting,
	}

	for _, ext := range e.extensions {
		onExecExt, ok := ext.(OnExecExtension)
		if ok {
			onExecExt.OnExec(path)
		}
	}

	exports, err := starlark.ExecFile(t, path, nil, e.predeclared)
	e.loadCache[path] = loadCacheEntry{
		status:  loadStatusDone,
		exports: exports,
		err:     err,
	}
	return exports, err
}

type ArgUnpacker func(fnName string, args starlark.Tuple, kwargs []starlark.Tuple, pairs ...interface{}) error

const (
	loadStatusNone loadStatus = iota
	loadStatusExecuting
	loadStatusDone
)

type loadCacheEntry struct {
	status  loadStatus
	exports starlark.StringDict
	err     error
}

type loadStatus int
