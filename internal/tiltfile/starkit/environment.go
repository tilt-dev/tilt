package starkit

import (
	"fmt"
	"path/filepath"
	"strings"

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
	unpackArgs     ArgUnpacker
	loadCache      map[string]loadCacheEntry
	predeclared    starlark.StringDict
	print          func(thread *starlark.Thread, msg string)
	extensions     []Extension
	fakeFileSystem map[string]string
}

func newEnvironment(extensions ...Extension) *Environment {
	return &Environment{
		unpackArgs:     starlark.UnpackArgs,
		loadCache:      make(map[string]loadCacheEntry),
		extensions:     append([]Extension{}, extensions...),
		predeclared:    starlark.StringDict{},
		fakeFileSystem: nil,
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
func (e *Environment) AddBuiltin(name string, b func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)) error {
	wrapped := starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		for _, ext := range e.extensions {
			onBuiltinCallExt, ok := ext.(OnBuiltinCallExtension)
			if ok {
				onBuiltinCallExt.OnBuiltinCall(name, fn)
			}
		}

		return b(thread, fn, args, kwargs)
	})

	return e.AddValue(name, wrapped)
}

func (e *Environment) AddValue(name string, val starlark.Value) error {
	split := strings.Split(name, ".")

	// Handle the simple case first.
	if len(split) == 1 {
		e.predeclared[name] = val
		return nil
	}

	if len(split) == 2 {
		var currentModule Module
		predeclaredVal, ok := e.predeclared[split[0]]
		if ok {
			predeclaredDict, ok := predeclaredVal.(Module)
			if !ok {
				return fmt.Errorf("Module conflict at %s. Existing: %s", name, predeclaredVal)
			}
			currentModule = predeclaredDict
		} else {
			currentModule = Module{name: split[0], attrs: starlark.StringDict{}}
			e.predeclared[split[0]] = currentModule
		}
		currentModule.attrs[split[1]] = val
		return nil
	}

	return fmt.Errorf("multi-level modules not supported yet")
}

func (e *Environment) SetPrint(print func(thread *starlark.Thread, msg string)) {
	e.print = print
}

// Set a fake file system so that we can write tests that don't
// touch the file system. Expressed as a map from paths to contents.
func (e *Environment) SetFakeFileSystem(files map[string]string) {
	e.fakeFileSystem = files
}

func (e *Environment) start(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, "environment#start")
	}

	for _, ext := range e.extensions {
		err := ext.OnStart(e)
		if err != nil {
			return errors.Wrapf(err, "%T", ext)
		}
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

	var contentBytes interface{} = nil
	if e.fakeFileSystem != nil {
		contents, ok := e.fakeFileSystem[path]
		if !ok {
			return starlark.StringDict{}, fmt.Errorf("Not in fake file system: %s", path)
		}
		contentBytes = []byte(contents)
	}

	exports, err := starlark.ExecFile(t, path, contentBytes, e.predeclared)
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
