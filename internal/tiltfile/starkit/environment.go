package starkit

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

// The main entrypoint to starkit.
// Execute a file with a set of starlark extensions.
func ExecFile(path string, extensions ...Extension) (Model, error) {
	return newEnvironment(extensions...).start(path)
}

const argUnpackerKey = "starkit.ArgUnpacker"
const modelKey = "starkit.Model"
const ctxKey = "starkit.Ctx"

// Unpacks args, using the arg unpacker on the current thread.
func UnpackArgs(t *starlark.Thread, fnName string, args starlark.Tuple, kwargs []starlark.Tuple, pairs ...interface{}) error {
	unpacker, ok := t.Local(argUnpackerKey).(ArgUnpacker)
	if !ok {
		return starlark.UnpackArgs(fnName, args, kwargs, pairs...)
	}
	return unpacker(fnName, args, kwargs, pairs...)
}

// A starlark execution environment.
type Environment struct {
	ctx              context.Context
	unpackArgs       ArgUnpacker
	loadCache        map[string]loadCacheEntry
	predeclared      starlark.StringDict
	print            func(thread *starlark.Thread, msg string)
	extensions       []Extension
	fakeFileSystem   map[string]string
	loadInterceptors []LoadInterceptor
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

func (e *Environment) AddLoadInterceptor(i LoadInterceptor) {
	e.loadInterceptors = append(e.loadInterceptors, i)
}

func (e *Environment) SetArgUnpacker(unpackArgs ArgUnpacker) {
	e.unpackArgs = unpackArgs
}

// Add a builtin to the environment.
//
// All builtins will be wrapped to invoke OnBuiltinCall on every extension.
//
// All builtins should use starkit.UnpackArgs to get instrumentation.
func (e *Environment) AddBuiltin(name string, f Function) error {
	wrapped := starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		for _, ext := range e.extensions {
			onBuiltinCallExt, ok := ext.(OnBuiltinCallExtension)
			if ok {
				onBuiltinCallExt.OnBuiltinCall(name, fn)
			}
		}

		return f(thread, fn, args, kwargs)
	})

	return e.AddValue(name, wrapped)
}

func (e *Environment) AddValue(name string, val starlark.Value) error {
	split := strings.Split(name, ".")

	// Handle the simple case first.
	if len(split) == 1 {
		if _, ok := e.predeclared[name]; ok {
			return fmt.Errorf("multiple values added named %s", name)
		}
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
		if _, ok := currentModule.attrs[split[1]]; ok {
			return fmt.Errorf("multiple values added named %s", name)
		}
		currentModule.attrs[split[1]] = val
		return nil
	}

	return fmt.Errorf("multi-level modules not supported yet")
}

func (e *Environment) SetPrint(print func(thread *starlark.Thread, msg string)) {
	e.print = print
}

func (e *Environment) SetContext(ctx context.Context) {
	e.ctx = ctx
}

// Set a fake file system so that we can write tests that don't
// touch the file system. Expressed as a map from paths to contents.
func (e *Environment) SetFakeFileSystem(files map[string]string) {
	e.fakeFileSystem = files
}

func (e *Environment) start(path string) (Model, error) {
	// NOTE(dmiller): we only call Abs here because it's the root of the stack
	path, err := filepath.Abs(path)
	if err != nil {
		return Model{}, errors.Wrap(err, "environment#start")
	}

	model := NewModel()
	for _, ext := range e.extensions {
		sExt, isStateful := ext.(StatefulExtension)
		if isStateful {
			err := model.createInitState(sExt)
			if err != nil {
				return Model{}, err
			}
		}
	}

	for _, ext := range e.extensions {
		err := ext.OnStart(e)
		if err != nil {
			return Model{}, errors.Wrapf(err, "internal error: %T", ext)
		}
	}

	t := &starlark.Thread{
		Print: e.print,
		Load:  e.load,
	}
	t.SetLocal(argUnpackerKey, e.unpackArgs)
	t.SetLocal(modelKey, model)
	t.SetLocal(ctxKey, e.ctx)

	_, err = e.exec(t, path)
	return model, err
}

func (e *Environment) load(t *starlark.Thread, path string) (starlark.StringDict, error) {
	return e.exec(t, path)
}

func (e *Environment) exec(t *starlark.Thread, path string) (starlark.StringDict, error) {
	localPath, err := e.getPath(t, path)
	if err != nil {
		e.loadCache[localPath] = loadCacheEntry{
			status:  loadStatusDone,
			exports: starlark.StringDict{},
			err:     err,
		}
		return starlark.StringDict{}, err
	}

	entry := e.loadCache[localPath]
	switch entry.status {
	case loadStatusExecuting:
		return starlark.StringDict{}, fmt.Errorf("Circular load: %s", localPath)
	case loadStatusDone:
		return entry.exports, entry.err
	}

	e.loadCache[localPath] = loadCacheEntry{
		status: loadStatusExecuting,
	}

	exports, err := e.doLoad(t, localPath)
	e.loadCache[localPath] = loadCacheEntry{
		status:  loadStatusDone,
		exports: exports,
		err:     err,
	}
	return exports, err
}

func (e *Environment) getPath(t *starlark.Thread, path string) (string, error) {
	for _, i := range e.loadInterceptors {
		newPath, err := i.LocalPath(t, path)
		if err != nil {
			return "", err
		}
		if newPath != "" {
			// we found an interceptor that does something with this path, return early
			return newPath, nil
		}
	}

	return AbsPath(t, path), nil
}

func (e *Environment) doLoad(t *starlark.Thread, localPath string) (starlark.StringDict, error) {
	for _, ext := range e.extensions {
		onExecExt, ok := ext.(OnExecExtension)
		if ok {
			err := onExecExt.OnExec(t, localPath)
			if err != nil {
				return starlark.StringDict{}, err
			}
		}
	}

	var contentBytes interface{} = nil
	if e.fakeFileSystem != nil {
		contents, ok := e.fakeFileSystem[localPath]
		if !ok {
			return starlark.StringDict{}, fmt.Errorf("Not in fake file system: %s", localPath)
		}
		contentBytes = []byte(contents)
	}

	return starlark.ExecFile(t, localPath, contentBytes, e.predeclared)
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
