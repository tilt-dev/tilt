package starkit

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func init() {
	resolve.AllowSet = true
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
	resolve.AllowGlobalReassign = true
	resolve.AllowRecursion = true
}

// The main entrypoint to starkit.
// Execute a file with a set of starlark plugins.
func ExecFile(tf *v1alpha1.Tiltfile, plugins ...Plugin) (Model, error) {
	return newEnvironment(plugins...).start(tf)
}

const argUnpackerKey = "starkit.ArgUnpacker"
const modelKey = "starkit.Model"
const ctxKey = "starkit.Ctx"
const startTfKey = "starkit.StartTiltfile"
const execingTiltfileKey = "starkit.ExecingTiltfile"

// Unpacks args, using the arg unpacker on the current thread.
func UnpackArgs(t *starlark.Thread, fnName string, args starlark.Tuple, kwargs []starlark.Tuple, pairs ...interface{}) error {
	unpacker, ok := t.Local(argUnpackerKey).(ArgUnpacker)
	if !ok {
		return starlark.UnpackArgs(fnName, args, kwargs, pairs...)
	}
	return unpacker(fnName, args, kwargs, pairs...)
}

type BuiltinCall struct {
	Name string
	Args starlark.Tuple
	Dur  time.Duration
}

// A starlark execution environment.
type Environment struct {
	ctx              context.Context
	startTf          *v1alpha1.Tiltfile
	unpackArgs       ArgUnpacker
	loadCache        map[string]loadCacheEntry
	predeclared      starlark.StringDict
	print            func(thread *starlark.Thread, msg string)
	plugins          []Plugin
	fakeFileSystem   map[string]string
	loadInterceptors []LoadInterceptor

	builtinCalls []BuiltinCall
}

func newEnvironment(plugins ...Plugin) *Environment {
	return &Environment{
		unpackArgs:     starlark.UnpackArgs,
		loadCache:      make(map[string]loadCacheEntry),
		plugins:        append([]Plugin{}, plugins...),
		predeclared:    starlark.StringDict{},
		fakeFileSystem: nil,
		builtinCalls:   []BuiltinCall{},
	}
}

func (e *Environment) Predeclared() starlark.StringDict {
	return e.predeclared
}

func (e *Environment) AddLoadInterceptor(i LoadInterceptor) {
	e.loadInterceptors = append(e.loadInterceptors, i)
}

func (e *Environment) SetArgUnpacker(unpackArgs ArgUnpacker) {
	e.unpackArgs = unpackArgs
}

// The tiltfile model driving this environment.
func (e *Environment) StartTiltfile() *v1alpha1.Tiltfile {
	return e.startTf
}

// Add a builtin to the environment.
//
// All builtins will be wrapped to invoke OnBuiltinCall on every plugin.
//
// All builtins should use starkit.UnpackArgs to get instrumentation.
func (e *Environment) AddBuiltin(name string, f Function) error {
	wrapped := starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		for _, ext := range e.plugins {
			onBuiltinCallExt, ok := ext.(OnBuiltinCallPlugin)
			if ok {
				onBuiltinCallExt.OnBuiltinCall(name, fn)
			}
		}

		start := time.Now()
		defer func() {
			e.builtinCalls = append(e.builtinCalls, BuiltinCall{
				Name: name,
				Args: args,
				Dur:  time.Since(start),
			})
		}()
		return f(thread, fn, args, kwargs)
	})

	return e.AddValue(name, wrapped)
}

func (e *Environment) AddValue(name string, val starlark.Value) error {
	split := strings.Split(name, ".")

	var attrMap = e.predeclared

	// Iterate thru the module tree.
	for i := 0; i < len(split)-1; i++ {
		var currentModule Module
		currentPart := split[i]
		fullName := strings.Join(split[:i+1], ".")
		predeclaredVal, ok := attrMap[currentPart]
		if ok {
			predeclaredDict, ok := predeclaredVal.(Module)
			if !ok {
				return fmt.Errorf("Module conflict at %s. Existing: %s", fullName, predeclaredVal)
			}
			currentModule = predeclaredDict
		} else {
			currentModule = Module{fullName: fullName, attrs: starlark.StringDict{}}
			attrMap[currentPart] = currentModule
		}

		attrMap = currentModule.attrs
	}

	baseName := split[len(split)-1]
	if _, ok := attrMap[baseName]; ok {
		return fmt.Errorf("multiple values added named %s", name)
	}
	attrMap[baseName] = val
	return nil
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

func (e *Environment) start(tf *v1alpha1.Tiltfile) (Model, error) {
	// NOTE(dmiller): we only call Abs here because it's the root of the stack
	path, err := filepath.Abs(tf.Spec.Path)
	if err != nil {
		return Model{}, errors.Wrap(err, "environment#start")
	}

	e.startTf = tf

	model := NewModel()
	for _, ext := range e.plugins {
		sExt, isStateful := ext.(StatefulPlugin)
		if isStateful {
			err := model.createInitState(sExt)
			if err != nil {
				return Model{}, err
			}
		}
	}

	for _, ext := range e.plugins {
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
	t.SetLocal(startTfKey, e.startTf)

	_, err = e.exec(t, path)
	model.BuiltinCalls = e.builtinCalls
	if errors.Is(err, ErrStopExecution) {
		return model, nil
	}
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

	oldPath := t.Local(execingTiltfileKey)
	t.SetLocal(execingTiltfileKey, localPath)

	exports, err := e.doLoad(t, localPath)

	t.SetLocal(execingTiltfileKey, oldPath)

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
	var bytes []byte
	if e.fakeFileSystem != nil {
		contents, ok := e.fakeFileSystem[localPath]
		if !ok {
			return starlark.StringDict{}, fmt.Errorf("Not in fake file system: %s", localPath)
		}
		bytes = []byte(contents)
	} else {
		var err error
		bytes, err = ioutil.ReadFile(localPath)
		if err != nil {
			return starlark.StringDict{}, fmt.Errorf("error reading file %s: %w", localPath, err)
		}
	}

	for _, ext := range e.plugins {
		onExecExt, ok := ext.(OnExecPlugin)
		if ok {
			err := onExecExt.OnExec(t, localPath, bytes)
			if err != nil {
				return starlark.StringDict{}, err
			}
		}
	}

	// Create a copy of predeclared variables so we can specify Tiltfile-specific values.
	predeclared := starlark.StringDict{}
	for k, v := range e.predeclared {
		predeclared[k] = v
	}
	predeclared["__file__"] = starlark.String(localPath)

	return starlark.ExecFile(t, localPath, bytes, predeclared)
}

type ArgUnpacker func(fnName string, args starlark.Tuple, kwargs []starlark.Tuple, pairs ...interface{}) error

const (
	loadStatusNone loadStatus = iota
	loadStatusExecuting
	loadStatusDone
)

var _ loadStatus = loadStatusNone

type loadCacheEntry struct {
	status  loadStatus
	exports starlark.StringDict
	err     error
}

type loadStatus int
